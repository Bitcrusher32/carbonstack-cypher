package db

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCreateGetAndListRelaySpaces(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)

	created, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-1",
		DisplayLabel:       "  test relay space  ",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
		CreatedAt:          "2026-06-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("create relay space: %v", err)
	}

	if created.RelaySpaceID != "relay-space-1" {
		t.Fatalf("relay_space_id = %q, want relay-space-1", created.RelaySpaceID)
	}
	if created.DisplayLabel != "test relay space" {
		t.Fatalf("display_label = %q, want trimmed label", created.DisplayLabel)
	}
	if created.CreatedByAccountID != "account-1" {
		t.Fatalf("created_by_account_id = %q, want account-1", created.CreatedByAccountID)
	}
	if created.CreatedByDeviceID != "device-1" {
		t.Fatalf("created_by_device_id = %q, want device-1", created.CreatedByDeviceID)
	}
	if created.CreatedAt != "2026-06-08T00:00:00Z" {
		t.Fatalf("created_at = %q", created.CreatedAt)
	}

	got, err := store.GetRelaySpace("relay-space-1")
	if err != nil {
		t.Fatalf("get relay space: %v", err)
	}
	if got != created {
		t.Fatalf("got relay space = %+v, want %+v", got, created)
	}

	spaces, err := store.ListRelaySpaces()
	if err != nil {
		t.Fatalf("list relay spaces: %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("len(spaces) = %d, want 1", len(spaces))
	}
	if spaces[0].RelaySpaceID != "relay-space-1" {
		t.Fatalf("spaces[0].relay_space_id = %q", spaces[0].RelaySpaceID)
	}
}

func TestGetRelaySpaceNotFound(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	_, err := store.GetRelaySpace("missing")
	if !errors.Is(err, ErrRelaySpaceNotFound) {
		t.Fatalf("err = %v, want ErrRelaySpaceNotFound", err)
	}
}

func TestCreateRelaySpaceInviteAndLookupByTokenHash(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	space := createRelaySpaceForHelperTest(t, store)
	member := registerRelaySpaceMemberForHelperTest(t, store, space.RelaySpaceID)

	maxClaims := 2
	created, err := store.CreateRelaySpaceInvite(CreateRelaySpaceInviteInput{
		RelaySpaceInviteID: "invite-1",
		RelaySpaceID:       space.RelaySpaceID,
		InviteToken:        "secret relay space token",
		DisplayCode:        "8F3A-C91B-2D44",
		WordCode:           "banana-wall-red-applesauce",
		CreatedByMemberID:  member.RoutingMemberID,
		CreatedAt:          "2026-06-08T00:00:00Z",
		MaxClaims:          &maxClaims,
		Note:               "  routing-only invite  ",
	})
	if err != nil {
		t.Fatalf("create relay space invite: %v", err)
	}

	wantHash := HashInviteCode("secret relay space token")
	if created.InviteTokenHash != wantHash {
		t.Fatalf("invite_token_hash = %q, want %q", created.InviteTokenHash, wantHash)
	}
	if created.State != RelaySpaceInviteStateActive {
		t.Fatalf("state = %q, want active", created.State)
	}
	if created.ClaimCount != 0 {
		t.Fatalf("claim_count = %d, want 0", created.ClaimCount)
	}
	if created.MaxClaims == nil || *created.MaxClaims != 2 {
		t.Fatalf("max_claims = %v, want 2", created.MaxClaims)
	}
	if created.Note != "routing-only invite" {
		t.Fatalf("note = %q, want trimmed note", created.Note)
	}

	got, err := store.GetRelaySpaceInviteByTokenHash(wantHash)
	if err != nil {
		t.Fatalf("get relay space invite by token hash: %v", err)
	}
	if got.RelaySpaceInviteID != "invite-1" {
		t.Fatalf("relay_space_invite_id = %q, want invite-1", got.RelaySpaceInviteID)
	}
	if got.WordCode != "banana-wall-red-applesauce" {
		t.Fatalf("word_code = %q", got.WordCode)
	}
}

func TestGetRelaySpaceInviteByTokenHashNotFound(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	_, err := store.GetRelaySpaceInviteByTokenHash("missing")
	if !errors.Is(err, ErrRelaySpaceInviteNotFound) {
		t.Fatalf("err = %v, want ErrRelaySpaceInviteNotFound", err)
	}
}

func TestRegisterGetAndListRelaySpaceMembers(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	space := createRelaySpaceForHelperTest(t, store)

	member, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "routing-member-1",
		RelaySpaceID:    space.RelaySpaceID,
		AccountID:       "account-1",
		DeviceID:        "device-1",
		DisplayLabel:    "  alice routing member  ",
		JoinedAt:        "2026-06-08T00:00:00Z",
		LastSeenAt:      "2026-06-08T00:01:00Z",
	})
	if err != nil {
		t.Fatalf("register relay space member: %v", err)
	}

	if member.RoutingMemberID != "routing-member-1" {
		t.Fatalf("routing_member_id = %q", member.RoutingMemberID)
	}
	if member.State != RelaySpaceMemberStateActive {
		t.Fatalf("state = %q, want active", member.State)
	}
	if member.DisplayLabel != "alice routing member" {
		t.Fatalf("display_label = %q, want trimmed label", member.DisplayLabel)
	}
	if member.LastSeenAt != "2026-06-08T00:01:00Z" {
		t.Fatalf("last_seen_at = %q", member.LastSeenAt)
	}

	got, err := store.GetRelaySpaceMember("routing-member-1")
	if err != nil {
		t.Fatalf("get relay space member: %v", err)
	}
	if got != member {
		t.Fatalf("got relay space member = %+v, want %+v", got, member)
	}

	members, err := store.ListRelaySpaceMembers(space.RelaySpaceID)
	if err != nil {
		t.Fatalf("list relay space members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("len(members) = %d, want 1", len(members))
	}
	if members[0].RoutingMemberID != "routing-member-1" {
		t.Fatalf("members[0].routing_member_id = %q", members[0].RoutingMemberID)
	}
}

func TestGetRelaySpaceMemberNotFound(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	_, err := store.GetRelaySpaceMember("missing")
	if !errors.Is(err, ErrRelaySpaceMemberNotFound) {
		t.Fatalf("err = %v, want ErrRelaySpaceMemberNotFound", err)
	}
}

func TestCreateRelaySpaceValidatesCreatorAccountDevicePair(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	seedSecondRelaySpaceAccountAndDevice(t, store)

	tests := []struct {
		name      string
		accountID string
		deviceID  string
		wantErr   error
	}{
		{
			name:     "device requires account",
			deviceID: "device-1",
			wantErr:  ErrRelaySpaceAccountRequiredForDevice,
		},
		{
			name:      "account must exist",
			accountID: "missing-account",
			wantErr:   ErrRelaySpaceAccountNotFound,
		},
		{
			name:      "device must exist",
			accountID: "account-1",
			deviceID:  "missing-device",
			wantErr:   ErrRelaySpaceDeviceNotFound,
		},
		{
			name:      "device must belong to account",
			accountID: "account-1",
			deviceID:  "device-2",
			wantErr:   ErrRelaySpaceAccountDeviceMismatch,
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.CreateRelaySpace(CreateRelaySpaceInput{
				RelaySpaceID:       fmt.Sprintf("invalid-space-%d", index),
				DisplayLabel:       test.name,
				CreatedByAccountID: test.accountID,
				CreatedByDeviceID:  test.deviceID,
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
		})
	}

	var count int
	if err := store.DB.QueryRow(
		"SELECT COUNT(*) FROM relay_spaces",
	).Scan(&count); err != nil {
		t.Fatalf("count relay spaces: %v", err)
	}
	if count != 0 {
		t.Fatalf("relay space count = %d, want 0 after refused creates", count)
	}
}

func TestCreateRelaySpaceDoesNotAutoRegisterCreatorMember(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)

	space, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-no-auto-member",
		DisplayLabel:       "explicit membership boundary",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
	})
	if err != nil {
		t.Fatalf("create relay space: %v", err)
	}

	members, err := store.ListRelaySpaceMembers(space.RelaySpaceID)
	if err != nil {
		t.Fatalf("list relay space members: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("member count = %d, want 0 before explicit registration", len(members))
	}
}

func TestRegisterRelaySpaceMemberValidatesAccountDevicePair(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	seedSecondRelaySpaceAccountAndDevice(t, store)
	space := createRelaySpaceForHelperTest(t, store)

	tests := []struct {
		name      string
		accountID string
		deviceID  string
		wantErr   error
	}{
		{
			name:      "account must exist",
			accountID: "missing-account",
			wantErr:   ErrRelaySpaceAccountNotFound,
		},
		{
			name:      "device must exist",
			accountID: "account-1",
			deviceID:  "missing-device",
			wantErr:   ErrRelaySpaceDeviceNotFound,
		},
		{
			name:      "device must belong to account",
			accountID: "account-1",
			deviceID:  "device-2",
			wantErr:   ErrRelaySpaceAccountDeviceMismatch,
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
				RoutingMemberID: fmt.Sprintf("invalid-member-%d", index),
				RelaySpaceID:    space.RelaySpaceID,
				AccountID:       test.accountID,
				DeviceID:        test.deviceID,
				DisplayLabel:    test.name,
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
		})
	}

	accountOnly, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "account-only-member",
		RelaySpaceID:    space.RelaySpaceID,
		AccountID:       "account-1",
		DisplayLabel:    "account-only routing record",
	})
	if err != nil {
		t.Fatalf("register account-only member: %v", err)
	}
	if accountOnly.DeviceID != "" {
		t.Fatalf("device_id = %q, want empty account-only routing record", accountOnly.DeviceID)
	}
}

func TestCreateRelaySpaceInviteValidatesActiveSameSpaceCreator(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	seedSecondRelaySpaceAccountAndDevice(t, store)

	spaceOne, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-1",
		DisplayLabel:       "space one",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
	})
	if err != nil {
		t.Fatalf("create first relay space: %v", err)
	}

	spaceTwo, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-2",
		DisplayLabel:       "space two",
		CreatedByAccountID: "account-2",
		CreatedByDeviceID:  "device-2",
	})
	if err != nil {
		t.Fatalf("create second relay space: %v", err)
	}

	activeCreator, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "active-creator",
		RelaySpaceID:    spaceOne.RelaySpaceID,
		AccountID:       "account-1",
		DeviceID:        "device-1",
		DisplayLabel:    "active creator",
	})
	if err != nil {
		t.Fatalf("register active creator: %v", err)
	}

	wrongSpaceCreator, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "wrong-space-creator",
		RelaySpaceID:    spaceTwo.RelaySpaceID,
		AccountID:       "account-2",
		DeviceID:        "device-2",
		DisplayLabel:    "other space creator",
	})
	if err != nil {
		t.Fatalf("register wrong-space creator: %v", err)
	}

	inactiveCreator, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "inactive-creator",
		RelaySpaceID:    spaceOne.RelaySpaceID,
		AccountID:       "account-2",
		DisplayLabel:    "disabled account-only creator",
		State:           RelaySpaceMemberStateDisabled,
	})
	if err != nil {
		t.Fatalf("register inactive creator: %v", err)
	}

	tests := []struct {
		name      string
		creatorID string
		wantErr   error
	}{
		{
			name:      "creator must exist",
			creatorID: "missing-member",
			wantErr:   ErrRelaySpaceInviteCreatorNotFound,
		},
		{
			name:      "creator must belong to same relay space",
			creatorID: wrongSpaceCreator.RoutingMemberID,
			wantErr:   ErrRelaySpaceInviteCreatorWrongSpace,
		},
		{
			name:      "creator must be active",
			creatorID: inactiveCreator.RoutingMemberID,
			wantErr:   ErrRelaySpaceInviteCreatorInactive,
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.CreateRelaySpaceInvite(CreateRelaySpaceInviteInput{
				RelaySpaceInviteID: fmt.Sprintf("invalid-invite-%d", index),
				RelaySpaceID:       spaceOne.RelaySpaceID,
				InviteToken:        fmt.Sprintf("invalid-token-%d", index),
				DisplayCode:        fmt.Sprintf("INVALID-%d", index),
				CreatedByMemberID:  test.creatorID,
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
		})
	}

	invite, err := store.CreateRelaySpaceInvite(CreateRelaySpaceInviteInput{
		RelaySpaceInviteID: "valid-invite",
		RelaySpaceID:       spaceOne.RelaySpaceID,
		InviteToken:        "valid-token",
		DisplayCode:        "VALID-CODE",
		CreatedByMemberID:  activeCreator.RoutingMemberID,
	})
	if err != nil {
		t.Fatalf("create invite with active same-space creator: %v", err)
	}
	if invite.CreatedByMemberID != activeCreator.RoutingMemberID {
		t.Fatalf(
			"created_by_member_id = %q, want %q",
			invite.CreatedByMemberID,
			activeCreator.RoutingMemberID,
		)
	}
}

func TestRelaySpaceHelpersPreserveRoutingOnlyNamingBoundary(t *testing.T) {
	names := []string{
		"RelaySpace",
		"RelaySpaceInvite",
		"RelaySpaceMember",
		"RoutingMemberID",
		"CreateRelaySpace",
		"CreateRelaySpaceInvite",
		"RegisterRelaySpaceMember",
		"ListRelaySpaceMembers",
	}

	for _, name := range names {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "verified") {
			t.Fatalf("helper name %q must not imply verified identity", name)
		}
		if strings.Contains(lower, "trust") {
			t.Fatalf("helper name %q must not imply trust authority", name)
		}
	}
}

func seedSecondRelaySpaceAccountAndDevice(t *testing.T, store *Store) {
	t.Helper()

	_, err := store.DB.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		"account-2",
		"Bob",
		"2026-06-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert second account: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO devices (device_id, account_id, device_label, public_identity_key, public_prekey_bundle, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"device-2",
		"account-2",
		"bob-device",
		"bob-public-identity-key",
		"bob-prekey-bundle",
		"2026-06-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert second device: %v", err)
	}
}

func seedRelaySpaceAccountAndDevice(t *testing.T, store *Store) {
	t.Helper()

	_, err := store.DB.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		"account-1",
		"Alice",
		"2026-06-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO devices (device_id, account_id, device_label, public_identity_key, public_prekey_bundle, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"device-1",
		"account-1",
		"alice-device",
		"alice-public-identity-key",
		"alice-prekey-bundle",
		"2026-06-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
}

func createRelaySpaceForHelperTest(t *testing.T, store *Store) RelaySpace {
	t.Helper()

	space, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-1",
		DisplayLabel:       "test relay space",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
		CreatedAt:          "2026-06-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("create relay space fixture: %v", err)
	}

	return space
}

func registerRelaySpaceMemberForHelperTest(t *testing.T, store *Store, relaySpaceID string) RelaySpaceMember {
	t.Helper()

	member, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "routing-member-1",
		RelaySpaceID:    relaySpaceID,
		AccountID:       "account-1",
		DeviceID:        "device-1",
		DisplayLabel:    "alice routing member",
		JoinedAt:        "2026-06-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("register relay space member fixture: %v", err)
	}

	return member
}
