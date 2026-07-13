package httpapi_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/httpapi"
)

func TestRelaySpaceScopedInboxAndAckRequireActiveRecipientMembership(
	t *testing.T,
) {
	server, store := newDeliveryAuthorityTestServer(t)
	t.Cleanup(func() {
		server.Close()
	})

	alice := claimInvite(
		t,
		server.URL,
		"dev-invite",
		"alice-delivery-authority",
	)
	createDevInvite(
		t,
		server.URL,
		"bob-delivery-authority-invite",
	)
	bob := claimInvite(
		t,
		server.URL,
		"bob-delivery-authority-invite",
		"bob-delivery-authority",
	)

	aliceDevice := registerDevice(
		t,
		server.URL,
		alice.AccountID,
		"alice-delivery-authority-device",
		"stub-alice-delivery-authority-public-key",
		"stub-alice-delivery-authority-prekey",
	)
	bobDevice := registerDevice(
		t,
		server.URL,
		bob.AccountID,
		"bob-delivery-authority-device",
		"stub-bob-delivery-authority-public-key",
		"stub-bob-delivery-authority-prekey",
	)

	space := createRelaySpace(
		t,
		server.URL,
		map[string]any{
			"relay_space_id":        "relay-space-delivery-authority",
			"display_label":         "delivery authority",
			"created_by_account_id": alice.AccountID,
			"created_by_device_id":  aliceDevice.DeviceID,
		},
	)

	registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "alice-delivery-authority-member",
			"account_id":        alice.AccountID,
			"device_id":         aliceDevice.DeviceID,
			"display_label":     "Alice delivery authority",
			"state":             "active",
		},
	)
	registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "bob-delivery-authority-member",
			"account_id":        bob.AccountID,
			"device_id":         bobDevice.DeviceID,
			"display_label":     "Bob delivery authority",
			"state":             "active",
		},
	)

	firstEnvelope := submitRelaySpaceEnvelope(
		t,
		server.URL,
		space.RelaySpaceID,
		aliceDevice.DeviceID,
		bobDevice.DeviceID,
		base64.StdEncoding.EncodeToString(
			[]byte("queued before disable"),
		),
	)

	requireDeliveryAuthorityInboxEnvelope(
		t,
		server.URL,
		space.RelaySpaceID,
		bobDevice.DeviceID,
		firstEnvelope.EnvelopeID,
	)

	transitionDeliveryAuthorityMember(
		t,
		server.URL,
		space.RelaySpaceID,
		"bob-delivery-authority-member",
		"disabled",
		http.StatusOK,
	)

	requireDeliveryAuthorityRefusal(
		t,
		server.URL,
		space.RelaySpaceID,
		bobDevice.DeviceID,
		firstEnvelope.EnvelopeID,
	)
	requireEnvelopeDeliveryState(
		t,
		store,
		space.RelaySpaceID,
		firstEnvelope.EnvelopeID,
		"queued",
	)

	server.Close()
	server = httptest.NewServer(httpapi.New(store, true).Routes())

	requireDeliveryAuthorityRefusal(
		t,
		server.URL,
		space.RelaySpaceID,
		bobDevice.DeviceID,
		firstEnvelope.EnvelopeID,
	)
	requireEnvelopeDeliveryState(
		t,
		store,
		space.RelaySpaceID,
		firstEnvelope.EnvelopeID,
		"queued",
	)

	transitionDeliveryAuthorityMember(
		t,
		server.URL,
		space.RelaySpaceID,
		"bob-delivery-authority-member",
		"active",
		http.StatusOK,
	)

	requireDeliveryAuthorityInboxEnvelope(
		t,
		server.URL,
		space.RelaySpaceID,
		bobDevice.DeviceID,
		firstEnvelope.EnvelopeID,
	)
	ackDeliveryAuthorityEnvelope(
		t,
		server.URL,
		space.RelaySpaceID,
		firstEnvelope.EnvelopeID,
		bobDevice.DeviceID,
		http.StatusOK,
	)
	requireEnvelopeDeliveryState(
		t,
		store,
		space.RelaySpaceID,
		firstEnvelope.EnvelopeID,
		"acknowledged",
	)

	secondEnvelope := submitRelaySpaceEnvelope(
		t,
		server.URL,
		space.RelaySpaceID,
		aliceDevice.DeviceID,
		bobDevice.DeviceID,
		base64.StdEncoding.EncodeToString(
			[]byte("queued before leave"),
		),
	)

	transitionDeliveryAuthorityMember(
		t,
		server.URL,
		space.RelaySpaceID,
		"bob-delivery-authority-member",
		"left",
		http.StatusOK,
	)

	requireDeliveryAuthorityRefusal(
		t,
		server.URL,
		space.RelaySpaceID,
		bobDevice.DeviceID,
		secondEnvelope.EnvelopeID,
	)
	requireEnvelopeDeliveryState(
		t,
		store,
		space.RelaySpaceID,
		secondEnvelope.EnvelopeID,
		"queued",
	)

	server.Close()
	server = httptest.NewServer(httpapi.New(store, true).Routes())

	requireDeliveryAuthorityRefusal(
		t,
		server.URL,
		space.RelaySpaceID,
		bobDevice.DeviceID,
		secondEnvelope.EnvelopeID,
	)
	requireEnvelopeDeliveryState(
		t,
		store,
		space.RelaySpaceID,
		secondEnvelope.EnvelopeID,
		"queued",
	)
}

func newDeliveryAuthorityTestServer(
	t *testing.T,
) (*httptest.Server, *db.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cypher-delivery-authority.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := store.Migrate(migrationsDir); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := store.SeedDevInvite("dev-invite"); err != nil {
		t.Fatalf("seed dev invite: %v", err)
	}

	server := httptest.NewServer(httpapi.New(store, true).Routes())
	return server, store
}

func transitionDeliveryAuthorityMember(
	t *testing.T,
	serverURL string,
	relaySpaceID string,
	routingMemberID string,
	targetState string,
	expectedStatus int,
) {
	t.Helper()

	var response map[string]any
	doPost(
		t,
		serverURL+
			"/v0/relay-spaces/"+
			relaySpaceID+
			"/members/"+
			routingMemberID+
			"/state",
		map[string]any{
			"target_state": targetState,
		},
		expectedStatus,
		&response,
	)
}

func requireDeliveryAuthorityInboxEnvelope(
	t *testing.T,
	serverURL string,
	relaySpaceID string,
	deviceID string,
	envelopeID string,
) {
	t.Helper()

	var response struct {
		RelaySpaceID string `json:"relay_space_id"`
		Envelopes    []struct {
			EnvelopeID string `json:"envelope_id"`
		} `json:"envelopes"`
	}
	doGet(
		t,
		serverURL+
			"/v0/relay-spaces/"+
			relaySpaceID+
			"/devices/"+
			deviceID+
			"/envelopes",
		http.StatusOK,
		&response,
	)

	if response.RelaySpaceID != relaySpaceID {
		t.Fatalf(
			"inbox relay_space_id = %q, want %q",
			response.RelaySpaceID,
			relaySpaceID,
		)
	}
	if len(response.Envelopes) != 1 {
		t.Fatalf(
			"inbox envelope count = %d, want 1",
			len(response.Envelopes),
		)
	}
	if response.Envelopes[0].EnvelopeID != envelopeID {
		t.Fatalf(
			"inbox envelope_id = %q, want %q",
			response.Envelopes[0].EnvelopeID,
			envelopeID,
		)
	}
}

func requireDeliveryAuthorityRefusal(
	t *testing.T,
	serverURL string,
	relaySpaceID string,
	deviceID string,
	envelopeID string,
) {
	t.Helper()

	var inboxError errorResponse
	doGet(
		t,
		serverURL+
			"/v0/relay-spaces/"+
			relaySpaceID+
			"/devices/"+
			deviceID+
			"/envelopes",
		http.StatusForbidden,
		&inboxError,
	)
	if inboxError.Error.Code != "recipient_not_relay_member" {
		t.Fatalf(
			"inbox error = %q, want recipient_not_relay_member",
			inboxError.Error.Code,
		)
	}

	var ackError errorResponse
	doPost(
		t,
		serverURL+
			"/v0/relay-spaces/"+
			relaySpaceID+
			"/envelopes/"+
			envelopeID+
			"/ack",
		map[string]any{
			"recipient_device_id": deviceID,
		},
		http.StatusForbidden,
		&ackError,
	)
	if ackError.Error.Code != "recipient_not_relay_member" {
		t.Fatalf(
			"ack error = %q, want recipient_not_relay_member",
			ackError.Error.Code,
		)
	}
}

func ackDeliveryAuthorityEnvelope(
	t *testing.T,
	serverURL string,
	relaySpaceID string,
	envelopeID string,
	deviceID string,
	expectedStatus int,
) {
	t.Helper()

	var response struct {
		EnvelopeID    string `json:"envelope_id"`
		RelaySpaceID  string `json:"relay_space_id"`
		DeliveryState string `json:"delivery_state"`
	}
	doPost(
		t,
		serverURL+
			"/v0/relay-spaces/"+
			relaySpaceID+
			"/envelopes/"+
			envelopeID+
			"/ack",
		map[string]any{
			"recipient_device_id": deviceID,
		},
		expectedStatus,
		&response,
	)

	if expectedStatus == http.StatusOK {
		if response.EnvelopeID != envelopeID {
			t.Fatalf(
				"ack envelope_id = %q, want %q",
				response.EnvelopeID,
				envelopeID,
			)
		}
		if response.RelaySpaceID != relaySpaceID {
			t.Fatalf(
				"ack relay_space_id = %q, want %q",
				response.RelaySpaceID,
				relaySpaceID,
			)
		}
		if response.DeliveryState != "acknowledged" {
			t.Fatalf(
				"ack delivery_state = %q, want acknowledged",
				response.DeliveryState,
			)
		}
	}
}

func requireEnvelopeDeliveryState(
	t *testing.T,
	store *db.Store,
	relaySpaceID string,
	envelopeID string,
	expectedState string,
) {
	t.Helper()

	var state string
	err := store.DB.QueryRow(
		`SELECT delivery_state
		FROM envelopes
		WHERE relay_space_id = ? AND envelope_id = ?`,
		relaySpaceID,
		envelopeID,
	).Scan(&state)
	if err != nil {
		t.Fatalf("query envelope delivery_state: %v", err)
	}
	if state != expectedState {
		t.Fatalf(
			"delivery_state = %q, want %q",
			state,
			expectedState,
		)
	}
}
