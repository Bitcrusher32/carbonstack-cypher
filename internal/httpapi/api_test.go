package httpapi_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/httpapi"
)

func TestHealth(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	var resp map[string]string
	doGet(t, server.URL+"/v0/health", http.StatusOK, &resp)

	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", resp["status"])
	}
	if resp["service"] != "carbonstack-cypher" {
		t.Fatalf("expected service carbonstack-cypher, got %q", resp["service"])
	}
	if resp["api_version"] != "v0" {
		t.Fatalf("expected api_version v0, got %q", resp["api_version"])
	}
}

func TestFullRelayLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")

	bobInvite := createDevInvite(t, server.URL, "bob-test-invite")
	if bobInvite.InviteCode != "bob-test-invite" {
		t.Fatalf("expected bob-test-invite, got %q", bobInvite.InviteCode)
	}

	bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")

	aliceDevices := listDevices(t, server.URL, alice.AccountID)
	if len(aliceDevices.Devices) != 1 {
		t.Fatalf("expected 1 Alice device, got %d", len(aliceDevices.Devices))
	}
	if aliceDevices.Devices[0].DeviceID != aliceDevice.DeviceID {
		t.Fatalf("Alice device mismatch")
	}

	bobDevices := listDevices(t, server.URL, bob.AccountID)
	if len(bobDevices.Devices) != 1 {
		t.Fatalf("expected 1 Bob device, got %d", len(bobDevices.Devices))
	}
	if bobDevices.Devices[0].DeviceID != bobDevice.DeviceID {
		t.Fatalf("Bob device mismatch")
	}

	plaintext := "hello from cypher api test"
	ciphertextB64 := base64.StdEncoding.EncodeToString([]byte(plaintext))

	envelope := submitEnvelope(t, server.URL, aliceDevice.DeviceID, bobDevice.DeviceID, ciphertextB64)
	if envelope.DeliveryState != "queued" {
		t.Fatalf("expected queued envelope, got %q", envelope.DeliveryState)
	}
	if envelope.EnvelopeID == "" {
		t.Fatalf("expected envelope_id")
	}

	inbox := getInbox(t, server.URL, bobDevice.DeviceID)
	if len(inbox.Envelopes) != 1 {
		t.Fatalf("expected 1 queued envelope, got %d", len(inbox.Envelopes))
	}

	gotEnvelope := inbox.Envelopes[0]
	if gotEnvelope.EnvelopeID != envelope.EnvelopeID {
		t.Fatalf("expected envelope %s, got %s", envelope.EnvelopeID, gotEnvelope.EnvelopeID)
	}
	if gotEnvelope.SenderDeviceID != aliceDevice.DeviceID {
		t.Fatalf("sender device mismatch")
	}
	if gotEnvelope.RecipientDeviceID != bobDevice.DeviceID {
		t.Fatalf("recipient device mismatch")
	}

	decoded, err := base64.StdEncoding.DecodeString(gotEnvelope.CiphertextB64)
	if err != nil {
		t.Fatalf("decode ciphertext_b64: %v", err)
	}
	if string(decoded) != plaintext {
		t.Fatalf("expected plaintext %q, got %q", plaintext, string(decoded))
	}

	ack := ackEnvelope(t, server.URL, envelope.EnvelopeID, bobDevice.DeviceID)
	if ack.DeliveryState != "acknowledged" {
		t.Fatalf("expected acknowledged, got %q", ack.DeliveryState)
	}

	inboxAfterAck := getInbox(t, server.URL, bobDevice.DeviceID)
	if len(inboxAfterAck.Envelopes) != 0 {
		t.Fatalf("expected empty inbox after ack, got %d envelopes", len(inboxAfterAck.Envelopes))
	}
}

func TestAckIsIdempotentForRecipient(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")
	createDevInvite(t, server.URL, "bob-test-invite")
	bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")

	ciphertextB64 := base64.StdEncoding.EncodeToString([]byte("idempotent ack test"))
	envelope := submitEnvelope(t, server.URL, aliceDevice.DeviceID, bobDevice.DeviceID, ciphertextB64)

	firstAck := ackEnvelope(t, server.URL, envelope.EnvelopeID, bobDevice.DeviceID)
	if firstAck.DeliveryState != "acknowledged" {
		t.Fatalf("first ack delivery_state = %q, want acknowledged", firstAck.DeliveryState)
	}
	if firstAck.AcknowledgedAt == "" {
		t.Fatal("first ack acknowledged_at is empty")
	}

	secondAck := ackEnvelope(t, server.URL, envelope.EnvelopeID, bobDevice.DeviceID)
	if secondAck.DeliveryState != "acknowledged" {
		t.Fatalf("second ack delivery_state = %q, want acknowledged", secondAck.DeliveryState)
	}
	if secondAck.AcknowledgedAt != firstAck.AcknowledgedAt {
		t.Fatalf("second ack acknowledged_at = %q, want original %q", secondAck.AcknowledgedAt, firstAck.AcknowledgedAt)
	}

	inboxAfterAck := getInbox(t, server.URL, bobDevice.DeviceID)
	if len(inboxAfterAck.Envelopes) != 0 {
		t.Fatalf("expected empty inbox after idempotent ack, got %d envelopes", len(inboxAfterAck.Envelopes))
	}
}

func TestAckRejectsUnknownEnvelope(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")
	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")

	body := map[string]string{
		"recipient_device_id": aliceDevice.DeviceID,
	}

	var errResp errorResponse
	doPost(t, server.URL+"/v0/envelopes/not-a-real-envelope/ack", body, http.StatusNotFound, &errResp)

	if errResp.Error.Code != "envelope_not_found" {
		t.Fatalf("expected envelope_not_found, got %q", errResp.Error.Code)
	}
}

func TestAckRequiresRecipientDeviceID(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	var errResp errorResponse
	doPost(t, server.URL+"/v0/envelopes/not-a-real-envelope/ack", map[string]string{}, http.StatusBadRequest, &errResp)

	if errResp.Error.Code != "invalid_request" {
		t.Fatalf("expected invalid_request, got %q", errResp.Error.Code)
	}
}
func TestAckRejectsWrongRecipient(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")

	createDevInvite(t, server.URL, "bob-test-invite")
	bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

	createDevInvite(t, server.URL, "mallory-test-invite")
	mallory := claimInvite(t, server.URL, "mallory-test-invite", "mallory-test")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")
	malloryDevice := registerDevice(t, server.URL, mallory.AccountID, "mallory-cli-test", "stub-mallory-public-key", "stub-mallory-prekey")

	ciphertextB64 := base64.StdEncoding.EncodeToString([]byte("wrong recipient ack test"))
	envelope := submitEnvelope(t, server.URL, aliceDevice.DeviceID, bobDevice.DeviceID, ciphertextB64)

	body := map[string]string{
		"recipient_device_id": malloryDevice.DeviceID,
	}

	var errResp errorResponse
	doPost(t, server.URL+"/v0/envelopes/"+envelope.EnvelopeID+"/ack", body, http.StatusForbidden, &errResp)

	if errResp.Error.Code != "recipient_mismatch" {
		t.Fatalf("expected recipient_mismatch, got %q", errResp.Error.Code)
	}
}

type testServerState struct {
	store  *db.Store
	server *httptest.Server
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cypher-test.db")

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

	api := httpapi.New(store, true)
	return httptest.NewServer(api.Routes())
}

type claimInviteResponse struct {
	AccountID string `json:"account_id"`
	CreatedAt string `json:"created_at"`
}

type devInviteResponse struct {
	InviteID   string `json:"invite_id"`
	InviteCode string `json:"invite_code"`
	CreatedAt  string `json:"created_at"`
}

type registerDeviceResponse struct {
	DeviceID  string `json:"device_id"`
	AccountID string `json:"account_id"`
	CreatedAt string `json:"created_at"`
}

type listDevicesResponse struct {
	AccountID string `json:"account_id"`
	Devices   []struct {
		DeviceID           string `json:"device_id"`
		DeviceLabel        string `json:"device_label"`
		PublicIdentityKey  string `json:"public_identity_key"`
		PublicPrekeyBundle string `json:"public_prekey_bundle"`
		CreatedAt          string `json:"created_at"`
	} `json:"devices"`
}

type submitEnvelopeResponse struct {
	EnvelopeID       string `json:"envelope_id"`
	DeliveryState    string `json:"delivery_state"`
	ServerReceivedAt string `json:"server_received_at"`
}

type inboxResponse struct {
	DeviceID  string `json:"device_id"`
	Envelopes []struct {
		EnvelopeID        string `json:"envelope_id"`
		SenderDeviceID    string `json:"sender_device_id"`
		RecipientDeviceID string `json:"recipient_device_id"`
		ContentType       string `json:"content_type"`
		ProtocolVersion   string `json:"protocol_version"`
		CiphertextB64     string `json:"ciphertext_b64"`
		ClientCreatedAt   string `json:"client_created_at"`
		ServerReceivedAt  string `json:"server_received_at"`
		DeliveryState     string `json:"delivery_state"`
	} `json:"envelopes"`
}

type ackEnvelopeResponse struct {
	EnvelopeID     string `json:"envelope_id"`
	DeliveryState  string `json:"delivery_state"`
	AcknowledgedAt string `json:"acknowledged_at"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func createDevInvite(t *testing.T, serverURL string, inviteCode string) devInviteResponse {
	t.Helper()

	var resp devInviteResponse
	body := map[string]string{
		"invite_code": inviteCode,
	}

	doPost(t, serverURL+"/v0/dev/invites", body, http.StatusCreated, &resp)
	return resp
}

func claimInvite(t *testing.T, serverURL string, inviteCode string, displayName string) claimInviteResponse {
	t.Helper()

	var resp claimInviteResponse
	body := map[string]string{
		"invite_code":  inviteCode,
		"display_name": displayName,
	}

	doPost(t, serverURL+"/v0/invites/claim", body, http.StatusCreated, &resp)
	return resp
}

func registerDevice(t *testing.T, serverURL string, accountID string, label string, publicIdentityKey string, publicPrekeyBundle string) registerDeviceResponse {
	t.Helper()

	var resp registerDeviceResponse
	body := map[string]string{
		"account_id":           accountID,
		"device_label":         label,
		"public_identity_key":  publicIdentityKey,
		"public_prekey_bundle": publicPrekeyBundle,
	}

	doPost(t, serverURL+"/v0/devices/register", body, http.StatusCreated, &resp)
	return resp
}

func listDevices(t *testing.T, serverURL string, accountID string) listDevicesResponse {
	t.Helper()

	var resp listDevicesResponse
	doGet(t, serverURL+"/v0/accounts/"+accountID+"/devices", http.StatusOK, &resp)
	return resp
}

func submitEnvelope(t *testing.T, serverURL string, senderDeviceID string, recipientDeviceID string, ciphertextB64 string) submitEnvelopeResponse {
	t.Helper()

	return submitEnvelopeWithContentType(
		t,
		serverURL,
		senderDeviceID,
		recipientDeviceID,
		"carbonstack.message.text.stub.v0",
		"stub-v0",
		ciphertextB64,
	)
}

func submitEnvelopeWithContentType(t *testing.T, serverURL string, senderDeviceID string, recipientDeviceID string, contentType string, protocolVersion string, ciphertextB64 string) submitEnvelopeResponse {
	t.Helper()

	var resp submitEnvelopeResponse
	body := map[string]string{
		"sender_device_id":    senderDeviceID,
		"recipient_device_id": recipientDeviceID,
		"content_type":        contentType,
		"protocol_version":    protocolVersion,
		"ciphertext_b64":      ciphertextB64,
		"client_created_at":   "2026-05-21T00:00:00Z",
	}

	doPost(t, serverURL+"/v0/envelopes", body, http.StatusCreated, &resp)
	return resp
}
func getInbox(t *testing.T, serverURL string, deviceID string) inboxResponse {
	t.Helper()

	var resp inboxResponse
	doGet(t, serverURL+"/v0/devices/"+deviceID+"/envelopes", http.StatusOK, &resp)
	return resp
}

func ackEnvelope(t *testing.T, serverURL string, envelopeID string, recipientDeviceID string) ackEnvelopeResponse {
	t.Helper()

	var resp ackEnvelopeResponse
	body := map[string]string{
		"recipient_device_id": recipientDeviceID,
	}

	doPost(t, serverURL+"/v0/envelopes/"+envelopeID+"/ack", body, http.StatusOK, &resp)
	return resp
}

func assertPayloadMetadata(t *testing.T, gotSHA256 string, gotSizeBytes int64, payload []byte) {
	t.Helper()

	hash := sha256.Sum256(payload)
	wantSHA256 := hex.EncodeToString(hash[:])
	if gotSHA256 != wantSHA256 {
		t.Fatalf("payload_sha256 = %q, want %q", gotSHA256, wantSHA256)
	}
	if gotSizeBytes != int64(len(payload)) {
		t.Fatalf("payload_size_bytes = %d, want %d", gotSizeBytes, len(payload))
	}
}

func doPost(t *testing.T, url string, body any, expectedStatus int, out any) {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer resp.Body.Close()

	decodeHTTPResponse(t, resp, expectedStatus, out)
}

func doGet(t *testing.T, url string, expectedStatus int, out any) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()

	decodeHTTPResponse(t, resp, expectedStatus, out)
}

func decodeHTTPResponse(t *testing.T, resp *http.Response, expectedStatus int, out any) {
	t.Helper()

	if resp.StatusCode != expectedStatus {
		var errResp errorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("expected status %d, got %d, error=%+v", expectedStatus, resp.StatusCode, errResp)
	}

	if out == nil {
		return
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestRejectsInvalidBase64Envelope(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")
	createDevInvite(t, server.URL, "bob-test-invite")
	bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")

	body := map[string]string{
		"sender_device_id":    aliceDevice.DeviceID,
		"recipient_device_id": bobDevice.DeviceID,
		"content_type":        "carbonstack.message.text.stub.v0",
		"protocol_version":    "stub-v0",
		"ciphertext_b64":      "not base64 !!!",
		"client_created_at":   "2026-05-21T00:00:00Z",
	}

	var errResp errorResponse
	doPost(t, server.URL+"/v0/envelopes", body, http.StatusBadRequest, &errResp)

	if errResp.Error.Code != "invalid_ciphertext" {
		t.Fatalf("expected invalid_ciphertext, got %q", errResp.Error.Code)
	}
}

func TestRouteNotFoundForMalformedDeviceLookup(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	var errResp errorResponse
	doGet(t, server.URL+"/v0/accounts/not/a/valid/path/devices", http.StatusNotFound, &errResp)

	if errResp.Error.Code != "not_found" {
		t.Fatalf("expected not_found, got %q", errResp.Error.Code)
	}
}

func TestDevInviteCanAutogenerateCode(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	resp := createDevInvite(t, server.URL, "")
	if !strings.HasPrefix(resp.InviteCode, "dev-") {
		t.Fatalf("expected autogenerated dev invite, got %q", resp.InviteCode)
	}
}

func TestOpenMLSContentTypesRoundTripOpaqueBytes(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		payload     []byte
	}{
		{
			name:        "keypackage",
			contentType: "carbonstack.mls.keypackage.v0",
			payload:     []byte{0x00, 0x01, 0x02, 0x03, 0xfe, 0xff, 'k', 'p'},
		},
		{
			name:        "welcome",
			contentType: "carbonstack.mls.welcome.v0",
			payload:     []byte{0x77, 0x65, 0x6c, 0x63, 0x6f, 0x6d, 0x65, 0x00, 0xff},
		},
		{
			name:        "application message",
			contentType: "carbonstack.mls.application-message.v0",
			payload:     []byte{0x10, 0x20, 0x30, 0x00, 0x99, 0xaa, 0xbb, 0xcc},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTestServer(t)
			defer server.Close()

			alice := claimInvite(t, server.URL, "dev-invite", "alice-test")

			createDevInvite(t, server.URL, "bob-test-invite")
			bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

			aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
			bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")

			payloadB64 := base64.StdEncoding.EncodeToString(tt.payload)

			envelope := submitEnvelopeWithContentType(
				t,
				server.URL,
				aliceDevice.DeviceID,
				bobDevice.DeviceID,
				tt.contentType,
				"carbonstack-openmls-sidecar-v0",
				payloadB64,
			)

			if envelope.DeliveryState != "queued" {
				t.Fatalf("expected queued envelope, got %q", envelope.DeliveryState)
			}

			inbox := getInbox(t, server.URL, bobDevice.DeviceID)
			if len(inbox.Envelopes) != 1 {
				t.Fatalf("expected 1 queued envelope, got %d", len(inbox.Envelopes))
			}

			gotEnvelope := inbox.Envelopes[0]
			if gotEnvelope.ContentType != tt.contentType {
				t.Fatalf("content type = %q, want %q", gotEnvelope.ContentType, tt.contentType)
			}
			if gotEnvelope.ProtocolVersion != "carbonstack-openmls-sidecar-v0" {
				t.Fatalf("protocol version = %q, want carbonstack-openmls-sidecar-v0", gotEnvelope.ProtocolVersion)
			}

			gotPayload, err := base64.StdEncoding.DecodeString(gotEnvelope.CiphertextB64)
			if err != nil {
				t.Fatalf("decode ciphertext_b64: %v", err)
			}
			if !bytes.Equal(gotPayload, tt.payload) {
				t.Fatalf("payload roundtrip mismatch: got %x want %x", gotPayload, tt.payload)
			}
		})
	}
}

func TestOpenMLSContentTypeRejectsWrongProtocolVersion(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")

	createDevInvite(t, server.URL, "bob-test-invite")
	bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")

	body := map[string]string{
		"sender_device_id":    aliceDevice.DeviceID,
		"recipient_device_id": bobDevice.DeviceID,
		"content_type":        "carbonstack.mls.application-message.v0",
		"protocol_version":    "stub-v0",
		"ciphertext_b64":      base64.StdEncoding.EncodeToString([]byte("mls artifact bytes")),
		"client_created_at":   "2026-05-21T00:00:00Z",
	}

	var errResp errorResponse
	doPost(t, server.URL+"/v0/envelopes", body, http.StatusBadRequest, &errResp)

	if errResp.Error.Code != "unsupported_protocol_version" {
		t.Fatalf("expected unsupported_protocol_version, got %q", errResp.Error.Code)
	}
}

func TestStubContentTypeRejectsOpenMLSProtocolVersion(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")

	createDevInvite(t, server.URL, "bob-test-invite")
	bob := claimInvite(t, server.URL, "bob-test-invite", "bob-test")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-cli-test", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-cli-test", "stub-bob-public-key", "stub-bob-prekey")

	body := map[string]string{
		"sender_device_id":    aliceDevice.DeviceID,
		"recipient_device_id": bobDevice.DeviceID,
		"content_type":        "carbonstack.message.text.stub.v0",
		"protocol_version":    "carbonstack-openmls-sidecar-v0",
		"ciphertext_b64":      base64.StdEncoding.EncodeToString([]byte("stub bytes")),
		"client_created_at":   "2026-05-21T00:00:00Z",
	}

	var errResp errorResponse
	doPost(t, server.URL+"/v0/envelopes", body, http.StatusBadRequest, &errResp)

	if errResp.Error.Code != "unsupported_protocol_version" {
		t.Fatalf("expected unsupported_protocol_version, got %q", errResp.Error.Code)
	}
}

func TestRelaySpaceHTTPLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-test")
	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-relay-test", "stub-alice-public-key", "stub-alice-prekey")

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id":        "relay-space-1",
		"display_label":         "test relay space",
		"created_by_account_id": alice.AccountID,
		"created_by_device_id":  aliceDevice.DeviceID,
	})

	if space.RelaySpaceID != "relay-space-1" {
		t.Fatalf("relay_space_id = %q, want relay-space-1", space.RelaySpaceID)
	}
	if space.DisplayLabel != "test relay space" {
		t.Fatalf("display_label = %q, want test relay space", space.DisplayLabel)
	}
	if space.CreatedByAccountID != alice.AccountID {
		t.Fatalf("created_by_account_id = %q, want %q", space.CreatedByAccountID, alice.AccountID)
	}

	list := listRelaySpaces(t, server.URL)
	if len(list.RelaySpaces) != 1 {
		t.Fatalf("expected 1 relay space, got %d", len(list.RelaySpaces))
	}
	if list.RelaySpaces[0].RelaySpaceID != space.RelaySpaceID {
		t.Fatalf("list relay_space_id = %q, want %q", list.RelaySpaces[0].RelaySpaceID, space.RelaySpaceID)
	}

	got := getRelaySpace(t, server.URL, space.RelaySpaceID)
	if got.RelaySpaceID != space.RelaySpaceID {
		t.Fatalf("got relay_space_id = %q, want %q", got.RelaySpaceID, space.RelaySpaceID)
	}

	member := registerRelaySpaceMember(t, server.URL, space.RelaySpaceID, map[string]any{
		"routing_member_id": "routing-member-1",
		"account_id":        alice.AccountID,
		"device_id":         aliceDevice.DeviceID,
		"display_label":     "alice routing member",
	})

	if member.RoutingMemberID != "routing-member-1" {
		t.Fatalf("routing_member_id = %q, want routing-member-1", member.RoutingMemberID)
	}
	if member.State != "active" {
		t.Fatalf("member state = %q, want active", member.State)
	}

	members := listRelaySpaceMembers(t, server.URL, space.RelaySpaceID)
	if len(members.Members) != 1 {
		t.Fatalf("expected 1 relay space member, got %d", len(members.Members))
	}
	if members.Members[0].RoutingMemberID != member.RoutingMemberID {
		t.Fatalf("member list mismatch")
	}

	invite := createRelaySpaceInvite(t, server.URL, space.RelaySpaceID, map[string]any{
		"relay_space_invite_id": "relay-space-invite-1",
		"invite_token":          "secret relay space invite token",
		"display_code":          "8F3A-C91B-2D44",
		"word_code":             "banana-wall-red-applesauce",
		"created_by_member_id":  member.RoutingMemberID,
		"max_claims":            1,
		"note":                  "routing-only HTTP invite",
	})

	if invite.InviteToken != "secret relay space invite token" {
		t.Fatalf("invite_token = %q, want original token", invite.InviteToken)
	}
	if invite.RelaySpaceInvite.RelaySpaceInviteID != "relay-space-invite-1" {
		t.Fatalf("relay_space_invite_id = %q", invite.RelaySpaceInvite.RelaySpaceInviteID)
	}
	if invite.RelaySpaceInvite.RelaySpaceID != space.RelaySpaceID {
		t.Fatalf("invite relay_space_id = %q, want %q", invite.RelaySpaceInvite.RelaySpaceID, space.RelaySpaceID)
	}
	if invite.RelaySpaceInvite.State != "active" {
		t.Fatalf("invite state = %q, want active", invite.RelaySpaceInvite.State)
	}
	if invite.RelaySpaceInvite.WordCode != "banana-wall-red-applesauce" {
		t.Fatalf("word_code = %q", invite.RelaySpaceInvite.WordCode)
	}
}

func TestRelaySpaceHTTPRejectsMissingSpaceForSubresources(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	var errResp errorResponse
	doPost(t, server.URL+"/v0/relay-spaces/missing/members", map[string]any{
		"account_id": "account-1",
	}, http.StatusNotFound, &errResp)

	if errResp.Error.Code != "relay_space_not_found" {
		t.Fatalf("expected relay_space_not_found, got %q", errResp.Error.Code)
	}

	doPost(t, server.URL+"/v0/relay-spaces/missing/invites", map[string]any{
		"invite_token": "secret",
	}, http.StatusNotFound, &errResp)

	if errResp.Error.Code != "relay_space_not_found" {
		t.Fatalf("expected relay_space_not_found, got %q", errResp.Error.Code)
	}
}

func TestRelaySpaceHTTPDoesNotExposeTrustOrVerifiedFields(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id": "relay-space-1",
		"display_label":  "routing only",
	})

	payload, err := json.Marshal(space)
	if err != nil {
		t.Fatalf("marshal relay space response: %v", err)
	}

	lower := strings.ToLower(string(payload))
	if strings.Contains(lower, "trust") {
		t.Fatalf("relay space HTTP response must not expose trust authority fields: %s", string(payload))
	}
	if strings.Contains(lower, "verified") {
		t.Fatalf("relay space HTTP response must not expose verified authority fields: %s", string(payload))
	}
}

type relaySpaceHTTPResponse struct {
	RelaySpaceID       string `json:"relay_space_id"`
	DisplayLabel       string `json:"display_label"`
	CreatedByAccountID string `json:"created_by_account_id"`
	CreatedByDeviceID  string `json:"created_by_device_id"`
	CreatedAt          string `json:"created_at"`
	DisabledAt         string `json:"disabled_at"`
}

type relaySpacesListHTTPResponse struct {
	RelaySpaces []relaySpaceHTTPResponse `json:"relay_spaces"`
}

type relaySpaceMemberHTTPResponse struct {
	RoutingMemberID string `json:"routing_member_id"`
	RelaySpaceID    string `json:"relay_space_id"`
	AccountID       string `json:"account_id"`
	DeviceID        string `json:"device_id"`
	DisplayLabel    string `json:"display_label"`
	State           string `json:"state"`
	JoinedAt        string `json:"joined_at"`
	LastSeenAt      string `json:"last_seen_at"`
	DisabledAt      string `json:"disabled_at"`
}

type relaySpaceMembersListHTTPResponse struct {
	RelaySpaceID string                         `json:"relay_space_id"`
	Members      []relaySpaceMemberHTTPResponse `json:"members"`
}

type relaySpaceInviteHTTPResponse struct {
	RelaySpaceInviteID string `json:"relay_space_invite_id"`
	RelaySpaceID       string `json:"relay_space_id"`
	InviteTokenHash    string `json:"invite_token_hash"`
	DisplayCode        string `json:"display_code"`
	WordCode           string `json:"word_code"`
	CreatedByMemberID  string `json:"created_by_member_id"`
	CreatedAt          string `json:"created_at"`
	ExpiresAt          string `json:"expires_at"`
	MaxClaims          *int   `json:"max_claims"`
	ClaimCount         int    `json:"claim_count"`
	State              string `json:"state"`
	Note               string `json:"note"`
}

type createRelaySpaceInviteHTTPResponse struct {
	RelaySpaceInvite relaySpaceInviteHTTPResponse `json:"relay_space_invite"`
	InviteToken      string                       `json:"invite_token"`
}

func createRelaySpace(t *testing.T, serverURL string, body map[string]any) relaySpaceHTTPResponse {
	t.Helper()

	var resp relaySpaceHTTPResponse
	doPost(t, serverURL+"/v0/relay-spaces", body, http.StatusCreated, &resp)
	return resp
}

func listRelaySpaces(t *testing.T, serverURL string) relaySpacesListHTTPResponse {
	t.Helper()

	var resp relaySpacesListHTTPResponse
	doGet(t, serverURL+"/v0/relay-spaces", http.StatusOK, &resp)
	return resp
}

func getRelaySpace(t *testing.T, serverURL string, relaySpaceID string) relaySpaceHTTPResponse {
	t.Helper()

	var resp relaySpaceHTTPResponse
	doGet(t, serverURL+"/v0/relay-spaces/"+relaySpaceID, http.StatusOK, &resp)
	return resp
}

func registerRelaySpaceMember(t *testing.T, serverURL string, relaySpaceID string, body map[string]any) relaySpaceMemberHTTPResponse {
	t.Helper()

	var resp relaySpaceMemberHTTPResponse
	doPost(t, serverURL+"/v0/relay-spaces/"+relaySpaceID+"/members", body, http.StatusCreated, &resp)
	return resp
}

func listRelaySpaceMembers(t *testing.T, serverURL string, relaySpaceID string) relaySpaceMembersListHTTPResponse {
	t.Helper()

	var resp relaySpaceMembersListHTTPResponse
	doGet(t, serverURL+"/v0/relay-spaces/"+relaySpaceID+"/members", http.StatusOK, &resp)
	return resp
}

func createRelaySpaceInvite(t *testing.T, serverURL string, relaySpaceID string, body map[string]any) createRelaySpaceInviteHTTPResponse {
	t.Helper()

	var resp createRelaySpaceInviteHTTPResponse
	doPost(t, serverURL+"/v0/relay-spaces/"+relaySpaceID+"/invites", body, http.StatusCreated, &resp)
	return resp
}

func TestRelaySpaceScopedEnvelopeLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-scoped")
	createDevInvite(t, server.URL, "dev-invite-two")
	bob := claimInvite(t, server.URL, "dev-invite-two", "bob-scoped")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-scoped-device", "stub-alice-scoped-public-key", "stub-alice-scoped-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-scoped-device", "stub-bob-scoped-public-key", "stub-bob-scoped-prekey")

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id":        "relay-space-scoped",
		"display_label":         "scoped envelope space",
		"created_by_account_id": alice.AccountID,
		"created_by_device_id":  aliceDevice.DeviceID,
	})

	relayEnvelope := submitRelaySpaceEnvelope(t, server.URL, space.RelaySpaceID, aliceDevice.DeviceID, bobDevice.DeviceID, base64.StdEncoding.EncodeToString([]byte("scoped hello")))
	if relayEnvelope.RelaySpaceID != space.RelaySpaceID {
		t.Fatalf("relay_space_id = %q, want %q", relayEnvelope.RelaySpaceID, space.RelaySpaceID)
	}
	if relayEnvelope.DeliveryState != "queued" {
		t.Fatalf("delivery_state = %q, want queued", relayEnvelope.DeliveryState)
	}
	if relayEnvelope.PayloadSizeBytes != len([]byte("scoped hello")) {
		t.Fatalf("payload_size_bytes = %d", relayEnvelope.PayloadSizeBytes)
	}

	unscopedEnvelope := submitEnvelope(t, server.URL, aliceDevice.DeviceID, bobDevice.DeviceID, base64.StdEncoding.EncodeToString([]byte("unscoped hello")))

	scopedInbox := getRelaySpaceInbox(t, server.URL, space.RelaySpaceID, bobDevice.DeviceID)
	if len(scopedInbox.Envelopes) != 1 {
		t.Fatalf("scoped inbox len = %d, want 1", len(scopedInbox.Envelopes))
	}
	if scopedInbox.Envelopes[0].EnvelopeID != relayEnvelope.EnvelopeID {
		t.Fatalf("scoped inbox returned envelope %q, want %q", scopedInbox.Envelopes[0].EnvelopeID, relayEnvelope.EnvelopeID)
	}
	if scopedInbox.Envelopes[0].RelaySpaceID != space.RelaySpaceID {
		t.Fatalf("scoped inbox relay_space_id = %q, want %q", scopedInbox.Envelopes[0].RelaySpaceID, space.RelaySpaceID)
	}
	if scopedInbox.Envelopes[0].EnvelopeID == unscopedEnvelope.EnvelopeID {
		t.Fatal("scoped inbox must not return unscoped envelope")
	}

	ack := ackRelaySpaceEnvelope(t, server.URL, space.RelaySpaceID, relayEnvelope.EnvelopeID, bobDevice.DeviceID)
	if ack.RelaySpaceID != space.RelaySpaceID {
		t.Fatalf("ack relay_space_id = %q, want %q", ack.RelaySpaceID, space.RelaySpaceID)
	}
	if ack.DeliveryState != "acknowledged" {
		t.Fatalf("ack delivery_state = %q, want acknowledged", ack.DeliveryState)
	}

	scopedInbox = getRelaySpaceInbox(t, server.URL, space.RelaySpaceID, bobDevice.DeviceID)
	if len(scopedInbox.Envelopes) != 0 {
		t.Fatalf("scoped inbox len after ack = %d, want 0", len(scopedInbox.Envelopes))
	}
}

func TestRelaySpaceScopedEnvelopeRejectsWrongSpaceAndRecipient(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-wrong-space")
	createDevInvite(t, server.URL, "dev-invite-two")
	bob := claimInvite(t, server.URL, "dev-invite-two", "bob-wrong-space")
	createDevInvite(t, server.URL, "dev-invite-three")
	carol := claimInvite(t, server.URL, "dev-invite-three", "carol-wrong-space")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-device", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-device", "stub-bob-public-key", "stub-bob-prekey")
	carolDevice := registerDevice(t, server.URL, carol.AccountID, "carol-device", "stub-carol-public-key", "stub-carol-prekey")

	spaceA := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id": "relay-space-a",
		"display_label":  "space a",
	})
	spaceB := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id": "relay-space-b",
		"display_label":  "space b",
	})

	envelope := submitRelaySpaceEnvelope(t, server.URL, spaceA.RelaySpaceID, aliceDevice.DeviceID, bobDevice.DeviceID, base64.StdEncoding.EncodeToString([]byte("space a only")))

	var errResp errorResponse
	doGet(t, server.URL+"/v0/relay-spaces/"+spaceB.RelaySpaceID+"/devices/"+bobDevice.DeviceID+"/envelopes", http.StatusOK, &struct {
		RelaySpaceID string                       `json:"relay_space_id"`
		DeviceID     string                       `json:"device_id"`
		Envelopes    []relaySpaceEnvelopeResponse `json:"envelopes"`
	}{})

	doPost(t, server.URL+"/v0/relay-spaces/"+spaceB.RelaySpaceID+"/envelopes/"+envelope.EnvelopeID+"/ack", map[string]any{
		"recipient_device_id": bobDevice.DeviceID,
	}, http.StatusNotFound, &errResp)
	if errResp.Error.Code != "envelope_not_found" {
		t.Fatalf("wrong-space ack error = %q, want envelope_not_found", errResp.Error.Code)
	}

	doPost(t, server.URL+"/v0/relay-spaces/"+spaceA.RelaySpaceID+"/envelopes/"+envelope.EnvelopeID+"/ack", map[string]any{
		"recipient_device_id": carolDevice.DeviceID,
	}, http.StatusForbidden, &errResp)
	if errResp.Error.Code != "recipient_mismatch" {
		t.Fatalf("wrong-recipient ack error = %q, want recipient_mismatch", errResp.Error.Code)
	}
}

func TestRelaySpaceScopedEnvelopeRejectsMissingSpace(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-missing-space")
	createDevInvite(t, server.URL, "dev-invite-two")
	bob := claimInvite(t, server.URL, "dev-invite-two", "bob-missing-space")

	aliceDevice := registerDevice(t, server.URL, alice.AccountID, "alice-device", "stub-alice-public-key", "stub-alice-prekey")
	bobDevice := registerDevice(t, server.URL, bob.AccountID, "bob-device", "stub-bob-public-key", "stub-bob-prekey")

	var errResp errorResponse
	doPost(t, server.URL+"/v0/relay-spaces/missing/envelopes", map[string]any{
		"sender_device_id":    aliceDevice.DeviceID,
		"recipient_device_id": bobDevice.DeviceID,
		"content_type":        "carbonstack.message.text.stub.v0",
		"protocol_version":    "stub-v0",
		"ciphertext_b64":      base64.StdEncoding.EncodeToString([]byte("missing space")),
	}, http.StatusNotFound, &errResp)
	if errResp.Error.Code != "relay_space_not_found" {
		t.Fatalf("missing-space submit error = %q, want relay_space_not_found", errResp.Error.Code)
	}

	doGet(t, server.URL+"/v0/relay-spaces/missing/devices/"+bobDevice.DeviceID+"/envelopes", http.StatusNotFound, &errResp)
	if errResp.Error.Code != "relay_space_not_found" {
		t.Fatalf("missing-space inbox error = %q, want relay_space_not_found", errResp.Error.Code)
	}
}

type submitRelaySpaceEnvelopeResponse struct {
	EnvelopeID       string `json:"envelope_id"`
	RelaySpaceID     string `json:"relay_space_id"`
	DeliveryState    string `json:"delivery_state"`
	ServerReceivedAt string `json:"server_received_at"`
	PayloadSHA256    string `json:"payload_sha256"`
	PayloadSizeBytes int    `json:"payload_size_bytes"`
}

type relaySpaceEnvelopeResponse struct {
	EnvelopeID        string `json:"envelope_id"`
	RelaySpaceID      string `json:"relay_space_id"`
	SenderDeviceID    string `json:"sender_device_id"`
	RecipientDeviceID string `json:"recipient_device_id"`
	ContentType       string `json:"content_type"`
	ProtocolVersion   string `json:"protocol_version"`
	CiphertextB64     string `json:"ciphertext_b64"`
	PayloadSHA256     string `json:"payload_sha256"`
	PayloadSizeBytes  int64  `json:"payload_size_bytes"`
	ClientCreatedAt   string `json:"client_created_at"`
	ServerReceivedAt  string `json:"server_received_at"`
	DeliveryState     string `json:"delivery_state"`
}

type relaySpaceInboxResponse struct {
	RelaySpaceID string                       `json:"relay_space_id"`
	DeviceID     string                       `json:"device_id"`
	Envelopes    []relaySpaceEnvelopeResponse `json:"envelopes"`
}

type ackRelaySpaceEnvelopeResponse struct {
	EnvelopeID     string `json:"envelope_id"`
	RelaySpaceID   string `json:"relay_space_id"`
	DeliveryState  string `json:"delivery_state"`
	AcknowledgedAt string `json:"acknowledged_at"`
}

func submitRelaySpaceEnvelope(t *testing.T, serverURL string, relaySpaceID string, senderDeviceID string, recipientDeviceID string, ciphertextB64 string) submitRelaySpaceEnvelopeResponse {
	t.Helper()

	var resp submitRelaySpaceEnvelopeResponse
	doPost(t, serverURL+"/v0/relay-spaces/"+relaySpaceID+"/envelopes", map[string]any{
		"sender_device_id":    senderDeviceID,
		"recipient_device_id": recipientDeviceID,
		"content_type":        "carbonstack.message.text.stub.v0",
		"protocol_version":    "stub-v0",
		"ciphertext_b64":      ciphertextB64,
	}, http.StatusCreated, &resp)

	return resp
}

func getRelaySpaceInbox(t *testing.T, serverURL string, relaySpaceID string, deviceID string) relaySpaceInboxResponse {
	t.Helper()

	var resp relaySpaceInboxResponse
	doGet(t, serverURL+"/v0/relay-spaces/"+relaySpaceID+"/devices/"+deviceID+"/envelopes", http.StatusOK, &resp)
	return resp
}

func ackRelaySpaceEnvelope(t *testing.T, serverURL string, relaySpaceID string, envelopeID string, recipientDeviceID string) ackRelaySpaceEnvelopeResponse {
	t.Helper()

	var resp ackRelaySpaceEnvelopeResponse
	doPost(t, serverURL+"/v0/relay-spaces/"+relaySpaceID+"/envelopes/"+envelopeID+"/ack", map[string]any{
		"recipient_device_id": recipientDeviceID,
	}, http.StatusOK, &resp)
	return resp
}
