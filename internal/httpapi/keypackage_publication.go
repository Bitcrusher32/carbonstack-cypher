package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

const (
	keyPackagePublicationContentType     = "carbonstack.mls.keypackage.v0"
	keyPackagePublicationProtocolVersion = "carbonstack-openmls-sidecar-v0"
)

var keyPackagePublicationRefPattern = regexp.MustCompile(
	`^sha256:[0-9a-f]{64}$`,
)

type keyPackagePublicationRequest struct {
	SenderDeviceID    string `json:"sender_device_id"`
	RecipientDeviceID string `json:"recipient_device_id"`
	KeyPackageRef     string `json:"key_package_ref"`
	CiphertextB64     string `json:"ciphertext_b64"`
	ClientCreatedAt   string `json:"client_created_at,omitempty"`
}

type keyPackagePublicationResponse struct {
	EnvelopeID                string `json:"envelope_id"`
	RelaySpaceID              string `json:"relay_space_id"`
	SenderDeviceID            string `json:"sender_device_id"`
	RecipientDeviceID         string `json:"recipient_device_id"`
	KeyPackageRef             string `json:"key_package_ref"`
	ContentType               string `json:"content_type"`
	ProtocolVersion           string `json:"protocol_version"`
	DeliveryState             string `json:"delivery_state"`
	ServerReceivedAt          string `json:"server_received_at"`
	PayloadSHA256             string `json:"payload_sha256"`
	PayloadSizeBytes          int64  `json:"payload_size_bytes"`
	PublicationClassification string `json:"publication_classification"`
	Idempotent                bool   `json:"idempotent"`
}

func (a *API) publishRelaySpaceKeyPackage(
	w http.ResponseWriter,
	r *http.Request,
) {
	relaySpaceID := strings.TrimSpace(r.PathValue("relay_space_id"))
	if relaySpaceID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	var req keyPackagePublicationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.SenderDeviceID = strings.TrimSpace(req.SenderDeviceID)
	req.RecipientDeviceID = strings.TrimSpace(req.RecipientDeviceID)
	req.KeyPackageRef = strings.TrimSpace(req.KeyPackageRef)
	req.CiphertextB64 = strings.TrimSpace(req.CiphertextB64)
	req.ClientCreatedAt = strings.TrimSpace(req.ClientCreatedAt)

	if req.SenderDeviceID == "" ||
		req.RecipientDeviceID == "" ||
		req.KeyPackageRef == "" ||
		req.CiphertextB64 == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"sender_device_id, recipient_device_id, key_package_ref, and ciphertext_b64 are required",
		)
		return
	}
	if !keyPackagePublicationRefPattern.MatchString(req.KeyPackageRef) {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_key_package_ref",
			"key_package_ref must be sha256 followed by 64 lowercase hexadecimal characters",
		)
		return
	}
	if len(req.CiphertextB64) > 65536 {
		writeError(
			w,
			http.StatusBadRequest,
			"envelope_too_large",
			"ciphertext_b64 exceeds Phase 1 limit",
		)
		return
	}
	payload, err := base64.StdEncoding.DecodeString(req.CiphertextB64)
	if err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_ciphertext",
			"ciphertext_b64 must be valid base64",
		)
		return
	}
	digest := sha256.Sum256(payload)
	payloadSHA256 := hex.EncodeToString(digest[:])
	serverReceivedAt := db.NowUTC()

	result, err := a.store.PublishRelaySpaceKeyPackage(
		db.PublishRelaySpaceKeyPackageInput{
			RelaySpaceID:      relaySpaceID,
			SenderDeviceID:    req.SenderDeviceID,
			RecipientDeviceID: req.RecipientDeviceID,
			KeyPackageRef:     req.KeyPackageRef,
			ContentType:       keyPackagePublicationContentType,
			ProtocolVersion:   keyPackagePublicationProtocolVersion,
			CiphertextB64:     req.CiphertextB64,
			PayloadSHA256:     payloadSHA256,
			PayloadSizeBytes:  int64(len(payload)),
			ClientCreatedAt:   req.ClientCreatedAt,
			ServerReceivedAt:  serverReceivedAt,
		},
	)
	if err != nil {
		writeKeyPackagePublicationError(w, err)
		return
	}

	status := http.StatusCreated
	if result.Idempotent {
		status = http.StatusOK
	}
	publication := result.Publication
	writeJSON(
		w,
		status,
		keyPackagePublicationResponse{
			EnvelopeID:                publication.EnvelopeID,
			RelaySpaceID:              publication.RelaySpaceID,
			SenderDeviceID:            publication.SenderDeviceID,
			RecipientDeviceID:         publication.RecipientDeviceID,
			KeyPackageRef:             publication.KeyPackageRef,
			ContentType:               keyPackagePublicationContentType,
			ProtocolVersion:           keyPackagePublicationProtocolVersion,
			DeliveryState:             publication.DeliveryState,
			ServerReceivedAt:          publication.ServerReceivedAt,
			PayloadSHA256:             publication.PayloadSHA256,
			PayloadSizeBytes:          publication.PayloadSizeBytes,
			PublicationClassification: result.PublicationClassification,
			Idempotent:                result.Idempotent,
		},
	)
}

func writeKeyPackagePublicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrKeyPackagePublicationRelaySpaceNotFound):
		writeError(
			w,
			http.StatusNotFound,
			"relay_space_not_found",
			"relay space not found",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationSenderDeviceNotFound):
		writeError(
			w,
			http.StatusNotFound,
			"sender_device_not_found",
			"sender device not found",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationRecipientDeviceNotFound):
		writeError(
			w,
			http.StatusNotFound,
			"recipient_device_not_found",
			"recipient device not found",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationSenderNotMember):
		writeError(
			w,
			http.StatusForbidden,
			"sender_not_relay_member",
			"sender device is not an active member of this relay space",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationRecipientNotMember):
		writeError(
			w,
			http.StatusForbidden,
			"recipient_not_relay_member",
			"recipient device is not an active member of this relay space",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationReuseConflict):
		writeError(
			w,
			http.StatusConflict,
			"keypackage_publication_reuse_conflict",
			"KeyPackage publication is already bound to another destination",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationIdentityConflict):
		writeError(
			w,
			http.StatusConflict,
			"keypackage_publication_identity_conflict",
			"KeyPackage reference and serialized payload identity conflict",
		)
	case errors.Is(err, db.ErrKeyPackagePublicationContended):
		writeError(
			w,
			http.StatusConflict,
			"keypackage_publication_contended",
			"KeyPackage publication remained contended",
		)
	default:
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
	}
}
