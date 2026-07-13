package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

const (
	contentTypeMessageTextStub    = "carbonstack.message.text.stub.v0"
	contentTypeMLSKeyPackage      = "carbonstack.mls.keypackage.v0"
	contentTypeMLSWelcome         = "carbonstack.mls.welcome.v0"
	contentTypeMLSApplicationMsg  = "carbonstack.mls.application-message.v0"
	protocolVersionStubV0         = "stub-v0"
	protocolVersionOpenMLSSidecar = "carbonstack-openmls-sidecar-v0"
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
	mux.HandleFunc("POST /v0/relay-spaces", a.createRelaySpace)
	mux.HandleFunc("GET /v0/relay-spaces", a.listRelaySpaces)
	mux.HandleFunc("GET /v0/relay-spaces/{relay_space_id}", a.getRelaySpace)
	mux.HandleFunc("POST /v0/relay-spaces/{relay_space_id}/envelopes", a.submitRelaySpaceEnvelope)
	mux.HandleFunc("GET /v0/relay-spaces/{relay_space_id}/devices/{device_id}/envelopes", a.relaySpaceDeviceEnvelopes)
	mux.HandleFunc("POST /v0/relay-spaces/{relay_space_id}/envelopes/{envelope_id}/ack", a.ackRelaySpaceEnvelope)
	mux.HandleFunc("POST /v0/relay-spaces/{relay_space_id}/invites", a.createRelaySpaceInvite)
	mux.HandleFunc("POST /v0/relay-spaces/invites/claim", a.claimRelaySpaceInvite)
	mux.HandleFunc("POST /v0/relay-spaces/{relay_space_id}/members", a.registerRelaySpaceMember)
	mux.HandleFunc("GET /v0/relay-spaces/{relay_space_id}/members", a.listRelaySpaceMembers)
	mux.HandleFunc("POST /v0/relay-spaces/{relay_space_id}/members/{routing_member_id}/state", a.updateRelaySpaceMemberState)
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
	if !isSupportedContentType(req.ContentType) {
		writeError(w, http.StatusBadRequest, "unsupported_content_type", "unsupported content_type")
		return
	}

	if !isSupportedProtocolForContentType(req.ContentType, req.ProtocolVersion) {
		writeError(w, http.StatusBadRequest, "unsupported_protocol_version", "unsupported protocol_version")
		return
	}

	if len(req.CiphertextB64) > 65536 {
		writeError(w, http.StatusBadRequest, "envelope_too_large", "ciphertext_b64 exceeds Phase 1 limit")
		return
	}
	decodedPayload, err := base64.StdEncoding.DecodeString(req.CiphertextB64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ciphertext", "ciphertext_b64 must be valid base64")
		return
	}

	payloadHash := sha256.Sum256(decodedPayload)
	payloadSHA256 := hex.EncodeToString(payloadHash[:])
	payloadSizeBytes := len(decodedPayload)

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
	_, err = a.store.DB.Exec(
		"INSERT INTO envelopes (envelope_id, sender_device_id, recipient_device_id, content_type, protocol_version, ciphertext_b64, payload_sha256, payload_size_bytes, client_created_at, server_received_at, delivery_state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		envelopeID,
		req.SenderDeviceID,
		req.RecipientDeviceID,
		req.ContentType,
		req.ProtocolVersion,
		req.CiphertextB64,
		payloadSHA256,
		payloadSizeBytes,
		req.ClientCreatedAt,
		now,
		"queued",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"envelope_id":        envelopeID,
		"delivery_state":     "queued",
		"server_received_at": now,
		"payload_sha256":     payloadSHA256,
		"payload_size_bytes": payloadSizeBytes,
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
		"SELECT envelope_id, sender_device_id, recipient_device_id, content_type, protocol_version, ciphertext_b64, COALESCE(payload_sha256, ''), COALESCE(payload_size_bytes, 0), COALESCE(client_created_at, ''), server_received_at, delivery_state FROM envelopes WHERE recipient_device_id = ? AND relay_space_id IS NULL AND delivery_state = 'queued' ORDER BY server_received_at ASC",
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
		PayloadSHA256     string `json:"payload_sha256"`
		PayloadSizeBytes  int64  `json:"payload_size_bytes"`
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
			&e.PayloadSHA256,
			&e.PayloadSizeBytes,
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
	var deliveryState string
	var existingAcknowledgedAt string

	err := a.store.DB.QueryRow(
		"SELECT e.recipient_device_id, e.delivery_state, COALESCE((SELECT acknowledged_at FROM envelope_acks WHERE envelope_id = e.envelope_id AND recipient_device_id = e.recipient_device_id ORDER BY acknowledged_at ASC LIMIT 1), '') FROM envelopes e WHERE e.envelope_id = ?",
		envelopeID,
	).Scan(&actualRecipient, &deliveryState, &existingAcknowledgedAt)

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

	if deliveryState == "acknowledged" && existingAcknowledgedAt != "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"envelope_id":     envelopeID,
			"delivery_state":  "acknowledged",
			"acknowledged_at": existingAcknowledgedAt,
		})
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

	if existingAcknowledgedAt == "" {
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
	} else {
		now = existingAcknowledgedAt
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

func (a *API) submitRelaySpaceEnvelope(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	if relaySpaceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if _, err := a.store.GetRelaySpace(relaySpaceID); errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

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
	if !isSupportedContentType(req.ContentType) {
		writeError(w, http.StatusBadRequest, "unsupported_content_type", "unsupported content_type")
		return
	}
	if !isSupportedProtocolForContentType(req.ContentType, req.ProtocolVersion) {
		writeError(w, http.StatusBadRequest, "unsupported_protocol_version", "unsupported protocol_version")
		return
	}
	if len(req.CiphertextB64) > 65536 {
		writeError(w, http.StatusBadRequest, "envelope_too_large", "ciphertext_b64 exceeds Phase 1 limit")
		return
	}

	decodedPayload, err := base64.StdEncoding.DecodeString(req.CiphertextB64)
	if err != nil {
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

	senderIsMember, err := a.store.IsActiveRelaySpaceDeviceMember(
		relaySpaceID,
		req.SenderDeviceID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if !senderIsMember {
		writeError(
			w,
			http.StatusForbidden,
			"sender_not_relay_member",
			"sender device is not an active member of the relay space",
		)
		return
	}

	recipientIsMember, err := a.store.IsActiveRelaySpaceDeviceMember(
		relaySpaceID,
		req.RecipientDeviceID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if !recipientIsMember {
		writeError(
			w,
			http.StatusForbidden,
			"recipient_not_relay_member",
			"recipient device is not an active member of the relay space",
		)
		return
	}

	payloadHash := sha256.Sum256(decodedPayload)
	payloadSHA256 := hex.EncodeToString(payloadHash[:])
	payloadSizeBytes := len(decodedPayload)

	envelopeID := uuid.NewString()
	now := db.NowUTC()

	_, err = a.store.DB.Exec(
		"INSERT INTO envelopes (envelope_id, relay_space_id, sender_device_id, recipient_device_id, content_type, protocol_version, ciphertext_b64, payload_sha256, payload_size_bytes, client_created_at, server_received_at, delivery_state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		envelopeID,
		relaySpaceID,
		req.SenderDeviceID,
		req.RecipientDeviceID,
		req.ContentType,
		req.ProtocolVersion,
		req.CiphertextB64,
		payloadSHA256,
		payloadSizeBytes,
		req.ClientCreatedAt,
		now,
		"queued",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"envelope_id":        envelopeID,
		"relay_space_id":     relaySpaceID,
		"delivery_state":     "queued",
		"server_received_at": now,
		"payload_sha256":     payloadSHA256,
		"payload_size_bytes": payloadSizeBytes,
	})
}

func (a *API) relaySpaceDeviceEnvelopes(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	deviceID := strings.TrimSpace(r.PathValue("device_id"))

	if relaySpaceID == "" || deviceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if _, err := a.store.GetRelaySpace(relaySpaceID); errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	if !a.deviceExists(deviceID) {
		writeError(w, http.StatusNotFound, "device_not_found", "device not found")
		return
	}

	recipientIsMember, err := a.store.IsActiveRelaySpaceDeviceMember(
		relaySpaceID,
		deviceID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if !recipientIsMember {
		writeError(
			w,
			http.StatusForbidden,
			"recipient_not_relay_member",
			"recipient device is not an active member of the relay space",
		)
		return
	}

	rows, err := a.store.DB.Query(
		"SELECT envelope_id, relay_space_id, sender_device_id, recipient_device_id, content_type, protocol_version, ciphertext_b64, COALESCE(payload_sha256, ''), COALESCE(payload_size_bytes, 0), COALESCE(client_created_at, ''), server_received_at, delivery_state FROM envelopes WHERE relay_space_id = ? AND recipient_device_id = ? AND delivery_state = 'queued' ORDER BY server_received_at ASC",
		relaySpaceID,
		deviceID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()

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

	resp := struct {
		RelaySpaceID string                       `json:"relay_space_id"`
		DeviceID     string                       `json:"device_id"`
		Envelopes    []relaySpaceEnvelopeResponse `json:"envelopes"`
	}{
		RelaySpaceID: relaySpaceID,
		DeviceID:     deviceID,
		Envelopes:    []relaySpaceEnvelopeResponse{},
	}

	for rows.Next() {
		var e relaySpaceEnvelopeResponse
		if err := rows.Scan(
			&e.EnvelopeID,
			&e.RelaySpaceID,
			&e.SenderDeviceID,
			&e.RecipientDeviceID,
			&e.ContentType,
			&e.ProtocolVersion,
			&e.CiphertextB64,
			&e.PayloadSHA256,
			&e.PayloadSizeBytes,
			&e.ClientCreatedAt,
			&e.ServerReceivedAt,
			&e.DeliveryState,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		resp.Envelopes = append(resp.Envelopes, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *API) ackRelaySpaceEnvelope(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	envelopeID := strings.TrimSpace(r.PathValue("envelope_id"))

	if relaySpaceID == "" || envelopeID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if _, err := a.store.GetRelaySpace(relaySpaceID); errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
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

	var actualRelaySpaceID string
	var actualRecipient string
	var deliveryState string
	var existingAcknowledgedAt string

	err := a.store.DB.QueryRow(
		"SELECT COALESCE(e.relay_space_id, ''), e.recipient_device_id, e.delivery_state, COALESCE((SELECT acknowledged_at FROM envelope_acks WHERE envelope_id = e.envelope_id AND recipient_device_id = e.recipient_device_id ORDER BY acknowledged_at ASC LIMIT 1), '') FROM envelopes e WHERE e.envelope_id = ?",
		envelopeID,
	).Scan(&actualRelaySpaceID, &actualRecipient, &deliveryState, &existingAcknowledgedAt)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "envelope_not_found", "envelope not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if actualRelaySpaceID != relaySpaceID {
		writeError(w, http.StatusNotFound, "envelope_not_found", "envelope not found in relay space")
		return
	}
	if actualRecipient != req.RecipientDeviceID {
		writeError(w, http.StatusForbidden, "recipient_mismatch", "recipient_device_id does not match envelope recipient")
		return
	}

	recipientIsMember, err := a.store.IsActiveRelaySpaceDeviceMember(
		relaySpaceID,
		req.RecipientDeviceID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if !recipientIsMember {
		writeError(
			w,
			http.StatusForbidden,
			"recipient_not_relay_member",
			"recipient device is not an active member of the relay space",
		)
		return
	}

	if deliveryState == "acknowledged" && existingAcknowledgedAt != "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"envelope_id":     envelopeID,
			"relay_space_id":  relaySpaceID,
			"delivery_state":  "acknowledged",
			"acknowledged_at": existingAcknowledgedAt,
		})
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

	if existingAcknowledgedAt == "" {
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
	} else {
		now = existingAcknowledgedAt
	}

	if _, err := tx.Exec(
		"UPDATE envelopes SET delivery_state = 'acknowledged' WHERE envelope_id = ? AND relay_space_id = ?",
		envelopeID,
		relaySpaceID,
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
		"relay_space_id":  relaySpaceID,
		"delivery_state":  "acknowledged",
		"acknowledged_at": now,
	})
}

func writeRelaySpaceIntegrityError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, db.ErrRelaySpaceAccountRequiredForDevice):
		writeError(
			w,
			http.StatusBadRequest,
			"account_required_for_device",
			"account_id is required when device_id is supplied",
		)
	case errors.Is(err, db.ErrRelaySpaceAccountNotFound):
		writeError(w, http.StatusNotFound, "account_not_found", "account not found")
	case errors.Is(err, db.ErrRelaySpaceDeviceNotFound):
		writeError(w, http.StatusNotFound, "device_not_found", "device not found")
	case errors.Is(err, db.ErrRelaySpaceAccountDeviceMismatch):
		writeError(
			w,
			http.StatusConflict,
			"account_device_mismatch",
			"device does not belong to account",
		)
	case errors.Is(err, db.ErrRelaySpaceInviteCreatorNotFound):
		writeError(
			w,
			http.StatusNotFound,
			"relay_space_member_not_found",
			"invite creator routing member not found",
		)
	case errors.Is(err, db.ErrRelaySpaceInviteCreatorWrongSpace):
		writeError(
			w,
			http.StatusConflict,
			"invite_creator_wrong_space",
			"invite creator routing member belongs to another relay space",
		)
	case errors.Is(err, db.ErrRelaySpaceInviteCreatorInactive):
		writeError(
			w,
			http.StatusConflict,
			"invite_creator_not_active",
			"invite creator routing member is not active",
		)
	default:
		return false
	}

	return true
}

type createRelaySpaceRequest struct {
	RelaySpaceID       string `json:"relay_space_id"`
	DisplayLabel       string `json:"display_label"`
	CreatedByAccountID string `json:"created_by_account_id"`
	CreatedByDeviceID  string `json:"created_by_device_id"`
}

func (a *API) createRelaySpace(w http.ResponseWriter, r *http.Request) {
	var req createRelaySpaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	space, err := a.store.CreateRelaySpace(db.CreateRelaySpaceInput{
		RelaySpaceID:       req.RelaySpaceID,
		DisplayLabel:       req.DisplayLabel,
		CreatedByAccountID: req.CreatedByAccountID,
		CreatedByDeviceID:  req.CreatedByDeviceID,
	})
	if err != nil {
		if writeRelaySpaceIntegrityError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, space)
}

func (a *API) listRelaySpaces(w http.ResponseWriter, r *http.Request) {
	spaces, err := a.store.ListRelaySpaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"relay_spaces": spaces,
	})
}

func (a *API) getRelaySpace(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	if relaySpaceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	space, err := a.store.GetRelaySpace(relaySpaceID)
	if errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, space)
}

type createRelaySpaceInviteRequest struct {
	RelaySpaceInviteID string `json:"relay_space_invite_id"`
	InviteToken        string `json:"invite_token"`
	InviteTokenHash    string `json:"invite_token_hash"`
	DisplayCode        string `json:"display_code"`
	WordCode           string `json:"word_code"`
	CreatedByMemberID  string `json:"created_by_member_id"`
	ExpiresAt          string `json:"expires_at"`
	MaxClaims          *int   `json:"max_claims"`
	State              string `json:"state"`
	Note               string `json:"note"`
}

func (a *API) createRelaySpaceInvite(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	if relaySpaceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if _, err := a.store.GetRelaySpace(relaySpaceID); errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	var req createRelaySpaceInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.InviteToken = strings.TrimSpace(req.InviteToken)
	req.DisplayCode = strings.TrimSpace(req.DisplayCode)

	if req.InviteToken == "" && strings.TrimSpace(req.InviteTokenHash) == "" {
		req.InviteToken = uuid.NewString()
	}
	if req.DisplayCode == "" {
		req.DisplayCode = defaultRelaySpaceDisplayCode(req.InviteToken)
	}

	invite, err := a.store.CreateRelaySpaceInvite(db.CreateRelaySpaceInviteInput{
		RelaySpaceInviteID: req.RelaySpaceInviteID,
		RelaySpaceID:       relaySpaceID,
		InviteToken:        req.InviteToken,
		InviteTokenHash:    req.InviteTokenHash,
		DisplayCode:        req.DisplayCode,
		WordCode:           req.WordCode,
		CreatedByMemberID:  req.CreatedByMemberID,
		ExpiresAt:          req.ExpiresAt,
		MaxClaims:          req.MaxClaims,
		State:              req.State,
		Note:               req.Note,
	})
	if err != nil {
		if writeRelaySpaceIntegrityError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"relay_space_invite": invite,
		"invite_token":       req.InviteToken,
	})
}

type registerRelaySpaceMemberRequest struct {
	RoutingMemberID string `json:"routing_member_id"`
	AccountID       string `json:"account_id"`
	DeviceID        string `json:"device_id"`
	DisplayLabel    string `json:"display_label"`
	State           string `json:"state"`
	LastSeenAt      string `json:"last_seen_at"`
}

func (a *API) registerRelaySpaceMember(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	if relaySpaceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if _, err := a.store.GetRelaySpace(relaySpaceID); errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	var req registerRelaySpaceMemberRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.AccountID = strings.TrimSpace(req.AccountID)
	if req.AccountID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "account_id is required")
		return
	}

	member, err := a.store.RegisterRelaySpaceMember(db.RegisterRelaySpaceMemberInput{
		RoutingMemberID: req.RoutingMemberID,
		RelaySpaceID:    relaySpaceID,
		AccountID:       req.AccountID,
		DeviceID:        req.DeviceID,
		DisplayLabel:    req.DisplayLabel,
		State:           req.State,
		LastSeenAt:      req.LastSeenAt,
	})
	if err != nil {
		if writeRelaySpaceIntegrityError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

func (a *API) listRelaySpaceMembers(w http.ResponseWriter, r *http.Request) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	if relaySpaceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	if _, err := a.store.GetRelaySpace(relaySpaceID); errors.Is(err, db.ErrRelaySpaceNotFound) {
		writeError(w, http.StatusNotFound, "relay_space_not_found", "relay space not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	members, err := a.store.ListRelaySpaceMembers(relaySpaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"relay_space_id": relaySpaceID,
		"members":        members,
	})
}

func defaultRelaySpaceDisplayCode(inviteToken string) string {
	compact := strings.ToUpper(strings.ReplaceAll(inviteToken, "-", ""))
	if len(compact) >= 12 {
		return compact[:4] + "-" + compact[4:8] + "-" + compact[8:12]
	}
	if compact != "" {
		return compact
	}
	generated := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))
	return generated[:4] + "-" + generated[4:8] + "-" + generated[8:12]
}

func isSupportedContentType(contentType string) bool {
	switch contentType {
	case contentTypeMessageTextStub,
		contentTypeMLSKeyPackage,
		contentTypeMLSWelcome,
		contentTypeMLSApplicationMsg:
		return true
	default:
		return false
	}
}

func isSupportedProtocolForContentType(contentType string, protocolVersion string) bool {
	switch contentType {
	case contentTypeMessageTextStub:
		return protocolVersion == protocolVersionStubV0
	case contentTypeMLSKeyPackage, contentTypeMLSWelcome, contentTypeMLSApplicationMsg:
		return protocolVersion == protocolVersionOpenMLSSidecar
	default:
		return false
	}
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
