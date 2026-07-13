package httpapi_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"testing"
)

type keyPackagePublicationTestResponse struct {
	EnvelopeID                string `json:"envelope_id"`
	RelaySpaceID              string `json:"relay_space_id"`
	RecipientDeviceID         string `json:"recipient_device_id"`
	KeyPackageRef             string `json:"key_package_ref"`
	ContentType               string `json:"content_type"`
	ProtocolVersion           string `json:"protocol_version"`
	DeliveryState             string `json:"delivery_state"`
	PayloadSHA256             string `json:"payload_sha256"`
	PublicationClassification string `json:"publication_classification"`
	Idempotent                bool   `json:"idempotent"`
}

func TestKeyPackagePublicationHTTPCreateReplayAndConflicts(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "b5c-alice")
	createDevInvite(t, server.URL, "b5c-bob-invite")
	bob := claimInvite(t, server.URL, "b5c-bob-invite", "b5c-bob")
	createDevInvite(t, server.URL, "b5c-charlie-invite")
	charlie := claimInvite(
		t, server.URL, "b5c-charlie-invite", "b5c-charlie",
	)

	aliceDevice := registerDevice(
		t,
		server.URL,
		alice.AccountID,
		"b5c-alice-device",
		"b5c-alice-public",
		"b5c-alice-prekey",
	)
	bobDevice := registerDevice(
		t,
		server.URL,
		bob.AccountID,
		"b5c-bob-device",
		"b5c-bob-public",
		"b5c-bob-prekey",
	)
	charlieDevice := registerDevice(
		t,
		server.URL,
		charlie.AccountID,
		"b5c-charlie-device",
		"b5c-charlie-public",
		"b5c-charlie-prekey",
	)

	space := createRelaySpace(
		t,
		server.URL,
		map[string]any{
			"relay_space_id":        "b5c-publication-space",
			"display_label":         "B5c publication space",
			"created_by_account_id": alice.AccountID,
			"created_by_device_id":  aliceDevice.DeviceID,
		},
	)
	registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "b5c-alice-member",
			"account_id":        alice.AccountID,
			"device_id":         aliceDevice.DeviceID,
			"display_label":     "Alice B5c member",
			"state":             "active",
		},
	)
	registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "b5c-bob-member",
			"account_id":        bob.AccountID,
			"device_id":         bobDevice.DeviceID,
			"display_label":     "Bob B5c member",
			"state":             "active",
		},
	)
	registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "b5c-charlie-member",
			"account_id":        charlie.AccountID,
			"device_id":         charlieDevice.DeviceID,
			"display_label":     "Charlie B5c member",
			"state":             "active",
		},
	)

	payload := []byte("b5c-keypackage-artifact")
	payloadB64 := base64.StdEncoding.EncodeToString(payload)
	ref := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	body := map[string]string{
		"sender_device_id":    aliceDevice.DeviceID,
		"recipient_device_id": bobDevice.DeviceID,
		"key_package_ref":     ref,
		"ciphertext_b64":      payloadB64,
		"client_created_at":   "2026-07-13T05:00:00Z",
	}

	var created keyPackagePublicationTestResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+space.RelaySpaceID+
			"/keypackage-publications",
		body,
		http.StatusCreated,
		&created,
	)
	if created.PublicationClassification != "created" ||
		created.Idempotent ||
		created.EnvelopeID == "" ||
		created.ContentType != "carbonstack.mls.keypackage.v0" ||
		created.ProtocolVersion != "carbonstack-openmls-sidecar-v0" {
		t.Fatalf("created response = %+v", created)
	}
	digest := sha256.Sum256(payload)
	if created.PayloadSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("payload sha = %q", created.PayloadSHA256)
	}

	body["client_created_at"] = "2026-07-13T06:00:00Z"
	var replay keyPackagePublicationTestResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+space.RelaySpaceID+
			"/keypackage-publications",
		body,
		http.StatusOK,
		&replay,
	)
	if !replay.Idempotent ||
		replay.PublicationClassification != "already_published" ||
		replay.EnvelopeID != created.EnvelopeID {
		t.Fatalf("replay response = %+v", replay)
	}

	body["recipient_device_id"] = charlieDevice.DeviceID
	var reuse errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+space.RelaySpaceID+
			"/keypackage-publications",
		body,
		http.StatusConflict,
		&reuse,
	)
	if reuse.Error.Code != "keypackage_publication_reuse_conflict" {
		t.Fatalf("reuse error = %+v", reuse)
	}

	body["recipient_device_id"] = bobDevice.DeviceID
	body["ciphertext_b64"] = base64.StdEncoding.EncodeToString(
		[]byte("altered-keypackage"),
	)
	var identity errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+space.RelaySpaceID+
			"/keypackage-publications",
		body,
		http.StatusConflict,
		&identity,
	)
	if identity.Error.Code != "keypackage_publication_identity_conflict" {
		t.Fatalf("identity error = %+v", identity)
	}
}

func TestKeyPackagePublicationHTTPValidation(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	tests := []struct {
		name string
		body map[string]string
		code string
	}{
		{
			name: "invalid ref",
			body: map[string]string{
				"sender_device_id":    "sender",
				"recipient_device_id": "recipient",
				"key_package_ref":     "not-a-ref",
				"ciphertext_b64":      "aGVsbG8=",
			},
			code: "invalid_key_package_ref",
		},
		{
			name: "invalid payload",
			body: map[string]string{
				"sender_device_id":    "sender",
				"recipient_device_id": "recipient",
				"key_package_ref": "sha256:" +
					"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"ciphertext_b64": "!!!",
			},
			code: "invalid_ciphertext",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var response errorResponse
			doPost(
				t,
				server.URL+
					"/v0/relay-spaces/missing/keypackage-publications",
				test.body,
				http.StatusBadRequest,
				&response,
			)
			if response.Error.Code != test.code {
				t.Fatalf("error = %+v", response)
			}
		})
	}
}
