package db

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestKeyPackagePublicationCreatedReplayConflictsAndAckState(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()
	seedKeyPackagePublicationFixture(t, store, "primary")

	input := keyPackagePublicationTestInput(
		"publication-space-a",
		"publication-alice-device",
		"publication-bob-device",
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	)

	created, err := store.PublishRelaySpaceKeyPackage(input)
	if err != nil {
		t.Fatalf("create publication: %v", err)
	}
	if created.PublicationClassification != KeyPackagePublicationCreated ||
		created.Idempotent ||
		created.Publication.EnvelopeID == "" ||
		created.Publication.DeliveryState != "queued" {
		t.Fatalf("unexpected created result: %+v", created)
	}

	input.ClientCreatedAt = "2026-07-13T06:00:00Z"
	replay, err := store.PublishRelaySpaceKeyPackage(input)
	if err != nil {
		t.Fatalf("replay publication: %v", err)
	}
	if replay.PublicationClassification != KeyPackagePublicationAlreadyPublished ||
		!replay.Idempotent ||
		replay.Publication.EnvelopeID != created.Publication.EnvelopeID {
		t.Fatalf("unexpected replay: %+v", replay)
	}
	assertKeyPackagePublicationCounts(t, store, 1, 1)

	reuse := input
	reuse.RecipientDeviceID = "publication-charlie-device"
	if _, err := store.PublishRelaySpaceKeyPackage(reuse); !errors.Is(
		err, ErrKeyPackagePublicationReuseConflict,
	) {
		t.Fatalf("reuse error = %v", err)
	}
	assertKeyPackagePublicationCounts(t, store, 1, 1)

	crossSpace := input
	crossSpace.RelaySpaceID = "publication-space-b"
	if _, err := store.PublishRelaySpaceKeyPackage(crossSpace); !errors.Is(
		err, ErrKeyPackagePublicationReuseConflict,
	) {
		t.Fatalf("cross-space reuse error = %v", err)
	}
	assertKeyPackagePublicationCounts(t, store, 1, 1)

	identity := input
	identity.PayloadSHA256 =
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if _, err := store.PublishRelaySpaceKeyPackage(identity); !errors.Is(
		err, ErrKeyPackagePublicationIdentityConflict,
	) {
		t.Fatalf("reference identity error = %v", err)
	}
	assertKeyPackagePublicationCounts(t, store, 1, 1)

	payloadIdentity := input
	payloadIdentity.KeyPackageRef =
		"sha256:9999999999999999999999999999999999999999999999999999999999999999"
	if _, err := store.PublishRelaySpaceKeyPackage(payloadIdentity); !errors.Is(
		err, ErrKeyPackagePublicationIdentityConflict,
	) {
		t.Fatalf("payload identity error = %v", err)
	}
	assertKeyPackagePublicationCounts(t, store, 1, 1)

	if _, err := store.DB.Exec(
		"UPDATE envelopes SET delivery_state = 'acknowledged' WHERE ? IS NOT NULL AND envelope_id = ?",
		"2026-07-13T06:30:00Z",
		created.Publication.EnvelopeID,
	); err != nil {
		t.Fatalf("ack publication envelope fixture: %v", err)
	}
	afterAck, err := store.PublishRelaySpaceKeyPackage(input)
	if err != nil {
		t.Fatalf("replay after ack: %v", err)
	}
	if afterAck.Publication.EnvelopeID != created.Publication.EnvelopeID ||
		afterAck.Publication.DeliveryState != "acknowledged" ||
		!afterAck.Idempotent {
		t.Fatalf("replay after ack = %+v", afterAck)
	}
	assertKeyPackagePublicationCounts(t, store, 1, 1)
}

func TestKeyPackagePublicationConcurrentExactAndDestinationConflict(t *testing.T) {
	t.Run("exact", func(t *testing.T) {
		store := openMigratedRelaySpaceTestStore(t)
		defer store.Close()
		store.DB.SetMaxOpenConns(16)
		seedKeyPackagePublicationFixture(t, store, "exact")

		input := keyPackagePublicationTestInput(
			"publication-space-a",
			"publication-alice-device",
			"publication-bob-device",
			"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		)
		results := make(chan PublishRelaySpaceKeyPackageResult, 12)
		errs := make(chan error, 12)
		var wait sync.WaitGroup
		for index := 0; index < 12; index++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, err := store.PublishRelaySpaceKeyPackage(input)
				if err != nil {
					errs <- err
					return
				}
				results <- result
			}()
		}
		wait.Wait()
		close(results)
		close(errs)
		for err := range errs {
			t.Fatalf("concurrent exact publication: %v", err)
		}
		created := 0
		replayed := 0
		envelopeID := ""
		for result := range results {
			if envelopeID == "" {
				envelopeID = result.Publication.EnvelopeID
			}
			if result.Publication.EnvelopeID != envelopeID {
				t.Fatalf("concurrent envelope mismatch: %+v", result)
			}
			switch result.PublicationClassification {
			case KeyPackagePublicationCreated:
				created++
			case KeyPackagePublicationAlreadyPublished:
				replayed++
			default:
				t.Fatalf("classification = %q", result.PublicationClassification)
			}
		}
		if created != 1 || replayed != 11 {
			t.Fatalf("created=%d replayed=%d", created, replayed)
		}
		assertKeyPackagePublicationCounts(t, store, 1, 1)
	})

	t.Run("destinations", func(t *testing.T) {
		store := openMigratedRelaySpaceTestStore(t)
		defer store.Close()
		store.DB.SetMaxOpenConns(8)
		seedKeyPackagePublicationFixture(t, store, "destinations")

		base := keyPackagePublicationTestInput(
			"publication-space-a",
			"publication-alice-device",
			"publication-bob-device",
			"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			"1111111111111111111111111111111111111111111111111111111111111111",
		)
		inputs := []PublishRelaySpaceKeyPackageInput{
			base,
			func() PublishRelaySpaceKeyPackageInput {
				other := base
				other.RecipientDeviceID = "publication-charlie-device"
				return other
			}(),
		}
		errs := make(chan error, 2)
		results := make(chan PublishRelaySpaceKeyPackageResult, 2)
		var wait sync.WaitGroup
		for _, input := range inputs {
			input := input
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, err := store.PublishRelaySpaceKeyPackage(input)
				if err != nil {
					errs <- err
					return
				}
				results <- result
			}()
		}
		wait.Wait()
		close(errs)
		close(results)
		created := 0
		conflicts := 0
		for result := range results {
			if result.PublicationClassification == KeyPackagePublicationCreated {
				created++
			}
		}
		for err := range errs {
			if errors.Is(err, ErrKeyPackagePublicationReuseConflict) {
				conflicts++
				continue
			}
			t.Fatalf("destination race error: %v", err)
		}
		if created != 1 || conflicts != 1 {
			t.Fatalf("created=%d conflicts=%d", created, conflicts)
		}
		assertKeyPackagePublicationCounts(t, store, 1, 1)
	})
}

func TestKeyPackagePublicationRestartAndMembershipRefusal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "publication-restart.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	seedKeyPackagePublicationFixture(t, store, "restart")
	input := keyPackagePublicationTestInput(
		"publication-space-a",
		"publication-alice-device",
		"publication-bob-device",
		"sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"3333333333333333333333333333333333333333333333333333333333333333",
	)
	created, err := store.PublishRelaySpaceKeyPackage(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Migrate(filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	replay, err := reopened.PublishRelaySpaceKeyPackage(input)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Publication.EnvelopeID != created.Publication.EnvelopeID ||
		!replay.Idempotent {
		t.Fatalf("restart replay = %+v", replay)
	}

	if _, err := reopened.DB.Exec(
		"UPDATE relay_space_members SET state = 'disabled', disabled_at = ? WHERE device_id = ?",
		time.Now().UTC().Format(time.RFC3339),
		"publication-bob-device",
	); err != nil {
		t.Fatal(err)
	}
	newInput := keyPackagePublicationTestInput(
		"publication-space-a",
		"publication-alice-device",
		"publication-bob-device",
		"sha256:4444444444444444444444444444444444444444444444444444444444444444",
		"5555555555555555555555555555555555555555555555555555555555555555",
	)
	if _, err := reopened.PublishRelaySpaceKeyPackage(newInput); !errors.Is(
		err, ErrKeyPackagePublicationRecipientNotMember,
	) {
		t.Fatalf("inactive recipient error = %v", err)
	}
	assertKeyPackagePublicationCounts(t, reopened, 1, 1)

	if _, err := reopened.DB.Exec(
		"UPDATE relay_space_members SET state = 'active', disabled_at = NULL WHERE device_id = ?",
		"publication-bob-device",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.DB.Exec(
		"UPDATE relay_space_members SET state = 'disabled', disabled_at = ? WHERE device_id = ?",
		time.Now().UTC().Format(time.RFC3339),
		"publication-alice-device",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.PublishRelaySpaceKeyPackage(newInput); !errors.Is(
		err, ErrKeyPackagePublicationSenderNotMember,
	) {
		t.Fatalf("inactive sender error = %v", err)
	}
	assertKeyPackagePublicationCounts(t, reopened, 1, 1)
}

func TestKeyPackagePublicationMigrationSchema(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()
	if !relaySpaceTestTableExists(t, store, "keypackage_publications") {
		t.Fatal("keypackage_publications table missing")
	}
	columns := relaySpaceTestColumnNames(
		t, store, "keypackage_publications",
	)
	for _, required := range []string{
		"envelope_id",
		"sender_device_id",
		"key_package_ref",
		"payload_sha256",
		"relay_space_id",
		"recipient_device_id",
		"created_at",
	} {
		found := false
		for _, column := range columns {
			if column == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("required column %q missing: %v", required, columns)
		}
	}
}

func keyPackagePublicationTestInput(
	relaySpaceID string,
	senderDeviceID string,
	recipientDeviceID string,
	keyPackageRef string,
	payloadSHA256 string,
) PublishRelaySpaceKeyPackageInput {
	return PublishRelaySpaceKeyPackageInput{
		RelaySpaceID:      relaySpaceID,
		SenderDeviceID:    senderDeviceID,
		RecipientDeviceID: recipientDeviceID,
		KeyPackageRef:     keyPackageRef,
		ContentType:       "carbonstack.mls.keypackage.v0",
		ProtocolVersion:   "carbonstack-openmls-sidecar-v0",
		CiphertextB64:     "a2V5cGFja2FnZQ==",
		PayloadSHA256:     payloadSHA256,
		PayloadSizeBytes:  10,
		ClientCreatedAt:   "2026-07-13T05:00:00Z",
		ServerReceivedAt:  "2026-07-13T05:00:01Z",
	}
}

func seedKeyPackagePublicationFixture(
	t *testing.T,
	store *Store,
	label string,
) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, account := range []string{
		"publication-alice-account",
		"publication-bob-account",
		"publication-charlie-account",
	} {
		if _, err := store.DB.Exec(
			"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
			account,
			account+"-"+label,
			now,
		); err != nil {
			t.Fatalf("insert account %s: %v", account, err)
		}
	}
	devices := []struct {
		id      string
		account string
	}{
		{"publication-alice-device", "publication-alice-account"},
		{"publication-bob-device", "publication-bob-account"},
		{"publication-charlie-device", "publication-charlie-account"},
	}
	for _, device := range devices {
		if _, err := store.DB.Exec(
			`INSERT INTO devices (
			    device_id,
			    account_id,
			    device_label,
			    public_identity_key,
			    public_prekey_bundle,
			    created_at
			) VALUES (?, ?, ?, ?, ?, ?)`,
			device.id,
			device.account,
			device.id+"-"+label,
			"public-"+device.id,
			"prekey-"+device.id,
			now,
		); err != nil {
			t.Fatalf("insert device %s: %v", device.id, err)
		}
	}
	for _, space := range []string{
		"publication-space-a",
		"publication-space-b",
	} {
		if _, err := store.DB.Exec(
			`INSERT INTO relay_spaces (
			    relay_space_id,
			    display_label,
			    created_by_account_id,
			    created_by_device_id,
			    created_at
			) VALUES (?, ?, ?, ?, ?)`,
			space,
			space+"-"+label,
			"publication-alice-account",
			"publication-alice-device",
			now,
		); err != nil {
			t.Fatalf("insert space %s: %v", space, err)
		}
		for index, device := range devices {
			if _, err := store.DB.Exec(
				`INSERT INTO relay_space_members (
				    routing_member_id,
				    relay_space_id,
				    account_id,
				    device_id,
				    display_label,
				    state,
				    joined_at
				) VALUES (?, ?, ?, ?, ?, 'active', ?)`,
				fmt.Sprintf("%s-member-%d", space, index),
				space,
				device.account,
				device.id,
				device.id,
				now,
			); err != nil {
				t.Fatalf("insert member %s/%s: %v", space, device.id, err)
			}
		}
	}
}

func assertKeyPackagePublicationCounts(
	t *testing.T,
	store *Store,
	wantEnvelopes int,
	wantPublications int,
) {
	t.Helper()
	var envelopes int
	if err := store.DB.QueryRow(
		"SELECT COUNT(*) FROM envelopes",
	).Scan(&envelopes); err != nil {
		t.Fatal(err)
	}
	var publications int
	if err := store.DB.QueryRow(
		"SELECT COUNT(*) FROM keypackage_publications",
	).Scan(&publications); err != nil {
		t.Fatal(err)
	}
	if envelopes != wantEnvelopes || publications != wantPublications {
		t.Fatalf(
			"envelopes=%d publications=%d want=%d/%d",
			envelopes,
			publications,
			wantEnvelopes,
			wantPublications,
		)
	}
}
