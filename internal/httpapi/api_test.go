package httpapi_test

import (
	"bytes"
	"encoding/base64"
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

	var resp submitEnvelopeResponse
	body := map[string]string{
		"sender_device_id":    senderDeviceID,
		"recipient_device_id": recipientDeviceID,
		"content_type":        "carbonstack.message.text.stub.v0",
		"protocol_version":    "stub-v0",
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
