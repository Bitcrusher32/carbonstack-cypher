package httpapi

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

type API struct {
	store   *db.Store
	devMode bool
}

func New(store *db.Store, devMode bool) *API {
	return &API{
		store:   store,
		devMode: devMode,
	}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v0/health", a.health)
	mux.HandleFunc("POST /v0/dev/invites", a.createDevInvite)
	mux.HandleFunc("POST /v0/invites/claim", a.claimInvite)
	mux.HandleFunc("POST /v0/devices/register", a.registerDevice)
	mux.HandleFunc("GET /v0/accounts/", a.accountDevices)
	mux.HandleFunc("POST /v0/envelopes", a.submitEnvelope)
	mux.HandleFunc("GET /v0/devices/", a.deviceEnvelopes)
	mux.HandleFunc("POST /v0/envelopes/", a.ackEnvelope)

	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "ok",
		"service":     "carbonstack-cypher",
		"api_version": "v0",
	})
}

type createDevInviteRequest struct {
	InviteCode string `json:"invite_code"`
}

func (a *API) createDevInvite(w http.ResponseWriter, r *http.Request) {
	if !a.devMode {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	var req createDevInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if req.InviteCode == "" {
		req.InviteCode = "dev-" + uuid.NewString()
	}

	codeHash := db.HashInviteCode(req.InviteCode)

	var existing string
	err := a.store.DB.QueryRow(
		"SELECT invite_id FROM invites WHERE invite_code_hash = ? LIMIT 1",
		codeHash,
	).Scan(&existing)

	if err == nil {
		writeError(w, http.StatusConflict, "invite_exists", "invite code already exists")
		return
	}
	if err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	inviteID := uuid.NewString()
	now := db.NowUTC()

	_, err = a.store.DB.Exec(
		"INSERT INTO invites (invite_id, invite_code_hash, created_at) VALUES (?, ?, ?)",
		inviteID,
		codeHash,
		now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"invite_id":   inviteID,
		"invite_code": req.InviteCode,
		"created_at":  now,
	})
}

type claimInviteRequest struct {
	InviteCode  string `json:"invite_code"`
	DisplayName string `json:"display_name"`
}

func (a *API) claimInvite(w http.ResponseWriter, r *http.Request) {
	var req claimInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.InviteCode = strings.TrimSpace(req.InviteCode)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.InviteCode == "" || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "invite_code and display_name are required")
		return
	}

	tx, err := a.store.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer tx.Rollback()

	codeHash := db.HashInviteCode(req.InviteCode)

	var inviteID string
	var claimedAt sql.NullString
	var disabledAt sql.NullString

	err = tx.QueryRow(
		"SELECT invite_id, claimed_at, disabled_at FROM invites WHERE invite_code_hash = ? LIMIT 1",
		codeHash,
	).Scan(&inviteID, &claimedAt, &disabledAt)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "invalid_invite", "invite not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if claimedAt.Valid {
		writeError(w, http.StatusConflict, "invite_claimed", "invite already claimed")
		return
	}
	if disabledAt.Valid {
		writeError(w, http.StatusForbidden, "invite_disabled", "invite disabled")
		return
	}

	accountID := uuid.NewString()
	now := db.NowUTC()

	if _, err := tx.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		accountID,
		req.DisplayName,
		now,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	if _, err := tx.Exec(
		"UPDATE invites SET claimed_at = ?, claimed_by_account_id = ? WHERE invite_id = ?",
		now,
		accountID,
		inviteID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"account_id": accountID,
		"created_at": now,
	})
}

type registerDeviceRequest struct {
	AccountID          string `json:"account_id"`
	DeviceLabel        string `json:"device_label"`
	PublicIdentityKey  string `json:"public_identity_key"`
	PublicPrekeyBundle string `json:"public_prekey_bundle"`
}

func (a *API) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req registerDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.AccountID = strings.TrimSpace(req.AccountID)
	req.DeviceLabel = strings.TrimSpace(req.DeviceLabel)
	req.PublicIdentityKey = strings.TrimSpace(req.PublicIdentityKey)

	if req.AccountID == "" || req.DeviceLabel == "" || req.PublicIdentityKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "account_id, device_label, and public_identity_key are required")
		return
	}

	var accountID string
	err := a.store.DB.QueryRow(
		"SELECT account_id FROM accounts WHERE account_id = ? AND disabled_at IS NULL",
		req.AccountID,
	).Scan(&accountID)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "account_not_found", "account not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	deviceID := uuid.NewString()
	now := db.NowUTC()

	_, err = a.store.DB.Exec(
		"INSERT INTO devices (device_id, account_id, device_label, public_identity_key, public_prekey_bundle, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		deviceID,
		req.AccountID,
		req.DeviceLabel,
		req.PublicIdentityKey,
		req.PublicPrekeyBundle,
		now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"device_id":  deviceID,
		"account_id": req.AccountID,
		"created_at": now,
	})
}

func (a *API) accountDevices(w http.ResponseWriter, r *http.Request) {
	accountID, ok := pathBetween(r.URL.Path, "/v0/accounts/", "/devices")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	rows, err := a.store.DB.Query(
		"SELECT device_id, device_label, public_identity_key, COALESCE(public_prekey_bundle, ''), created_at FROM devices WHERE account_id = ? AND revoked_at IS NULL ORDER BY created_at ASC",
		accountID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()

	type deviceResponse struct {
		DeviceID           string `json:"device_id"`
		DeviceLabel        string `json:"device_label"`
		PublicIdentityKey  string `json:"public_identity_key"`
		PublicPrekeyBundle string `json:"public_prekey_bundle"`
		CreatedAt          string `json:"created_at"`
	}

	resp := struct {
		AccountID string           `json:"account_id"`
		Devices   []deviceResponse `json:"devices"`
	}{
		AccountID: accountID,
		Devices:   []deviceResponse{},
	}

	for rows.Next() {
		var d deviceResponse
		if err := rows.Scan(&d.DeviceID, &d.DeviceLabel, &d.PublicIdentityKey, &d.PublicPrekeyBundle, &d.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		resp.Devices = append(resp.Devices, d)
	}

	writeJSON(w, http.StatusOK, resp)
}

type submitEnvelopeRequest struct {
	SenderDeviceID    string `json:"sender_device_id"`
	RecipientDeviceID string `json:"recipient_device_id"`
	ContentType       string `json:"content_type"`
	ProtocolVersion   string `json:"protocol_version"`
	CiphertextB64     string `json:"ciphertext_b64"`
	ClientCreatedAt   string `json:"client_created_at"`
}

func (a *API) submitEnvelope(w http.ResponseWriter, r *http.Request) {
	var req submitEnvelopeRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.SenderDeviceID = strings.TrimSpace(req.SenderDeviceID)
	req.RecipientDeviceID = strings.TrimSpace(req.RecipientDeviceID)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.ProtocolVersion = strings.TrimSpace(req.ProtocolVersion)
	req.CiphertextB64 = strings.TrimSpace(req.CiphertextB64)

	if req.SenderDeviceID == "" || req.RecipientDeviceID == "" || req.ContentType == "" || req.ProtocolVersion == "" || req.CiphertextB64 == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "sender_device_id, recipient_device_id, content_type, protocol_version, and ciphertext_b64 are required")
		return
	}

	if req.ContentType != "carbonstack.message.text.stub.v0" {
		writeError(w, http.StatusBadRequest, "unsupported_content_type", "unsupported content_type")
		return
	}

	if req.ProtocolVersion != "stub-v0" {
		writeError(w, http.StatusBadRequest, "unsupported_protocol_version", "unsupported protocol_version")
		return
	}

	if len(req.CiphertextB64) > 65536 {
		writeError(w, http.StatusBadRequest, "envelope_too_large", "ciphertext_b64 exceeds Phase 1 limit")
		return
	}

	if _, err := base64.StdEncoding.DecodeString(req.CiphertextB64); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ciphertext", "ciphertext_b64 must be valid base64")
		return
	}

	if !a.deviceExists(req.SenderDeviceID) {
		writeError(w, http.StatusNotFound, "sender_device_not_found", "sender device not found")
		return
	}

	if !a.deviceExists(req.RecipientDeviceID) {
		writeError(w, http.StatusNotFound, "recipient_device_not_found", "recipient device not found")
		return
	}

	envelopeID := uuid.NewString()
	now := db.NowUTC()

	_, err := a.store.DB.Exec(
		"INSERT INTO envelopes (envelope_id, sender_device_id, recipient_device_id, content_type, protocol_version, ciphertext_b64, client_created_at, server_received_at, delivery_state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		envelopeID,
		req.SenderDeviceID,
		req.RecipientDeviceID,
		req.ContentType,
		req.ProtocolVersion,
		req.CiphertextB64,
		req.ClientCreatedAt,
		now,
		"queued",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"envelope_id":        envelopeID,
		"delivery_state":     "queued",
		"server_received_at": now,
	})
}

func (a *API) deviceEnvelopes(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := pathBetween(r.URL.Path, "/v0/devices/", "/envelopes")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if !a.deviceExists(deviceID) {
		writeError(w, http.StatusNotFound, "device_not_found", "device not found")
		return
	}

	rows, err := a.store.DB.Query(
		"SELECT envelope_id, sender_device_id, recipient_device_id, content_type, protocol_version, ciphertext_b64, COALESCE(client_created_at, ''), server_received_at, delivery_state FROM envelopes WHERE recipient_device_id = ? AND delivery_state = 'queued' ORDER BY server_received_at ASC",
		deviceID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()

	type envelopeResponse struct {
		EnvelopeID        string `json:"envelope_id"`
		SenderDeviceID    string `json:"sender_device_id"`
		RecipientDeviceID string `json:"recipient_device_id"`
		ContentType       string `json:"content_type"`
		ProtocolVersion   string `json:"protocol_version"`
		CiphertextB64     string `json:"ciphertext_b64"`
		ClientCreatedAt   string `json:"client_created_at"`
		ServerReceivedAt  string `json:"server_received_at"`
		DeliveryState     string `json:"delivery_state"`
	}

	resp := struct {
		DeviceID  string             `json:"device_id"`
		Envelopes []envelopeResponse `json:"envelopes"`
	}{
		DeviceID:  deviceID,
		Envelopes: []envelopeResponse{},
	}

	for rows.Next() {
		var e envelopeResponse
		if err := rows.Scan(
			&e.EnvelopeID,
			&e.SenderDeviceID,
			&e.RecipientDeviceID,
			&e.ContentType,
			&e.ProtocolVersion,
			&e.CiphertextB64,
			&e.ClientCreatedAt,
			&e.ServerReceivedAt,
			&e.DeliveryState,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		resp.Envelopes = append(resp.Envelopes, e)
	}

	writeJSON(w, http.StatusOK, resp)
}

type ackEnvelopeRequest struct {
	RecipientDeviceID string `json:"recipient_device_id"`
}

func (a *API) ackEnvelope(w http.ResponseWriter, r *http.Request) {
	envelopeID, ok := pathBetween(r.URL.Path, "/v0/envelopes/", "/ack")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	var req ackEnvelopeRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.RecipientDeviceID = strings.TrimSpace(req.RecipientDeviceID)
	if req.RecipientDeviceID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "recipient_device_id is required")
		return
	}

	var actualRecipient string
	err := a.store.DB.QueryRow(
		"SELECT recipient_device_id FROM envelopes WHERE envelope_id = ?",
		envelopeID,
	).Scan(&actualRecipient)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "envelope_not_found", "envelope not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if actualRecipient != req.RecipientDeviceID {
		writeError(w, http.StatusForbidden, "recipient_mismatch", "recipient_device_id does not match envelope recipient")
		return
	}

	ackID := uuid.NewString()
	now := db.NowUTC()

	tx, err := a.store.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"INSERT INTO envelope_acks (ack_id, envelope_id, recipient_device_id, acknowledged_at) VALUES (?, ?, ?, ?)",
		ackID,
		envelopeID,
		req.RecipientDeviceID,
		now,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	if _, err := tx.Exec(
		"UPDATE envelopes SET delivery_state = 'acknowledged' WHERE envelope_id = ?",
		envelopeID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"envelope_id":     envelopeID,
		"delivery_state":  "acknowledged",
		"acknowledged_at": now,
	})
}

func (a *API) deviceExists(deviceID string) bool {
	var found string
	err := a.store.DB.QueryRow(
		"SELECT device_id FROM devices WHERE device_id = ? AND revoked_at IS NULL",
		deviceID,
	).Scan(&found)
	return err == nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func pathBetween(path string, prefix string, suffix string) (string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	if !strings.HasSuffix(path, suffix) {
		return "", false
	}

	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	value = strings.Trim(value, "/")

	if value == "" || strings.Contains(value, "/") {
		return "", false
	}

	return value, true
}
