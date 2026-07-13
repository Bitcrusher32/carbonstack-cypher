package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	KeyPackagePublicationCreated          = "created"
	KeyPackagePublicationAlreadyPublished = "already_published"
)

var (
	ErrKeyPackagePublicationRelaySpaceNotFound = errors.New(
		"keypackage publication relay space not found",
	)
	ErrKeyPackagePublicationSenderDeviceNotFound = errors.New(
		"keypackage publication sender device not found",
	)
	ErrKeyPackagePublicationRecipientDeviceNotFound = errors.New(
		"keypackage publication recipient device not found",
	)
	ErrKeyPackagePublicationSenderNotMember = errors.New(
		"keypackage publication sender is not an active relay space member",
	)
	ErrKeyPackagePublicationRecipientNotMember = errors.New(
		"keypackage publication recipient is not an active relay space member",
	)
	ErrKeyPackagePublicationReuseConflict = errors.New(
		"keypackage publication is already bound to another destination",
	)
	ErrKeyPackagePublicationIdentityConflict = errors.New(
		"keypackage publication reference and payload identity conflict",
	)
	ErrKeyPackagePublicationContended = errors.New(
		"keypackage publication remained contended",
	)
	errKeyPackagePublicationRetry = errors.New(
		"retry keypackage publication",
	)
)

type PublishRelaySpaceKeyPackageInput struct {
	RelaySpaceID      string
	SenderDeviceID    string
	RecipientDeviceID string
	KeyPackageRef     string
	ContentType       string
	ProtocolVersion   string
	CiphertextB64     string
	PayloadSHA256     string
	PayloadSizeBytes  int64
	ClientCreatedAt   string
	ServerReceivedAt  string
}

type KeyPackagePublication struct {
	EnvelopeID        string
	SenderDeviceID    string
	KeyPackageRef     string
	PayloadSHA256     string
	RelaySpaceID      string
	RecipientDeviceID string
	CreatedAt         string
	DeliveryState     string
	ServerReceivedAt  string
	PayloadSizeBytes  int64
}

type PublishRelaySpaceKeyPackageResult struct {
	Publication               KeyPackagePublication
	PublicationClassification string
	Idempotent                bool
}

func (s *Store) PublishRelaySpaceKeyPackage(
	input PublishRelaySpaceKeyPackageInput,
) (PublishRelaySpaceKeyPackageResult, error) {
	input.RelaySpaceID = strings.TrimSpace(input.RelaySpaceID)
	input.SenderDeviceID = strings.TrimSpace(input.SenderDeviceID)
	input.RecipientDeviceID = strings.TrimSpace(input.RecipientDeviceID)
	input.KeyPackageRef = strings.TrimSpace(input.KeyPackageRef)
	input.ContentType = strings.TrimSpace(input.ContentType)
	input.ProtocolVersion = strings.TrimSpace(input.ProtocolVersion)
	input.CiphertextB64 = strings.TrimSpace(input.CiphertextB64)
	input.PayloadSHA256 = strings.TrimSpace(input.PayloadSHA256)
	input.ClientCreatedAt = strings.TrimSpace(input.ClientCreatedAt)
	input.ServerReceivedAt = strings.TrimSpace(input.ServerReceivedAt)

	if input.RelaySpaceID == "" ||
		input.SenderDeviceID == "" ||
		input.RecipientDeviceID == "" ||
		input.KeyPackageRef == "" ||
		input.ContentType == "" ||
		input.ProtocolVersion == "" ||
		input.CiphertextB64 == "" ||
		input.PayloadSHA256 == "" {
		return PublishRelaySpaceKeyPackageResult{},
			errors.New("keypackage publication input is incomplete")
	}
	if input.ServerReceivedAt == "" {
		input.ServerReceivedAt = NowUTC()
	}
	if input.ClientCreatedAt == "" {
		input.ClientCreatedAt = input.ServerReceivedAt
	}

	for attempt := 0; attempt < 30; attempt++ {
		result, err := s.publishRelaySpaceKeyPackageOnce(
			context.Background(),
			input,
		)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errKeyPackagePublicationRetry) &&
			!isSQLiteBusyError(err) {
			return PublishRelaySpaceKeyPackageResult{}, err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return PublishRelaySpaceKeyPackageResult{},
		ErrKeyPackagePublicationContended
}

func (s *Store) publishRelaySpaceKeyPackageOnce(
	ctx context.Context,
	input PublishRelaySpaceKeyPackageInput,
) (PublishRelaySpaceKeyPackageResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		if isSQLiteBusyError(err) {
			return PublishRelaySpaceKeyPackageResult{},
				errKeyPackagePublicationRetry
		}
		return PublishRelaySpaceKeyPackageResult{},
			fmt.Errorf("begin keypackage publication: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := requireKeyPackagePublicationRelaySpaceTx(
		ctx, tx, input.RelaySpaceID,
	); err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}
	if err := requireKeyPackagePublicationDeviceTx(
		ctx, tx, input.SenderDeviceID,
		ErrKeyPackagePublicationSenderDeviceNotFound,
	); err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}
	if err := requireKeyPackagePublicationDeviceTx(
		ctx, tx, input.RecipientDeviceID,
		ErrKeyPackagePublicationRecipientDeviceNotFound,
	); err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}
	if err := requireActiveKeyPackagePublicationMemberTx(
		ctx, tx, input.RelaySpaceID, input.SenderDeviceID,
		ErrKeyPackagePublicationSenderNotMember,
	); err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}
	if err := requireActiveKeyPackagePublicationMemberTx(
		ctx, tx, input.RelaySpaceID, input.RecipientDeviceID,
		ErrKeyPackagePublicationRecipientNotMember,
	); err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}

	byRef, err := keyPackagePublicationByRefTx(
		ctx, tx, input.SenderDeviceID, input.KeyPackageRef,
	)
	if err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}
	byPayload, err := keyPackagePublicationByPayloadTx(
		ctx, tx, input.SenderDeviceID, input.PayloadSHA256,
	)
	if err != nil {
		return PublishRelaySpaceKeyPackageResult{}, err
	}

	if byRef != nil && byPayload != nil &&
		byRef.EnvelopeID != byPayload.EnvelopeID {
		return PublishRelaySpaceKeyPackageResult{},
			ErrKeyPackagePublicationIdentityConflict
	}
	existing := byRef
	if existing == nil {
		existing = byPayload
	}
	if existing != nil {
		if existing.KeyPackageRef != input.KeyPackageRef ||
			existing.PayloadSHA256 != input.PayloadSHA256 {
			return PublishRelaySpaceKeyPackageResult{},
				ErrKeyPackagePublicationIdentityConflict
		}
		if existing.RelaySpaceID != input.RelaySpaceID ||
			existing.RecipientDeviceID != input.RecipientDeviceID {
			return PublishRelaySpaceKeyPackageResult{},
				ErrKeyPackagePublicationReuseConflict
		}
		_ = tx.Rollback()
		return PublishRelaySpaceKeyPackageResult{
			Publication:               *existing,
			PublicationClassification: KeyPackagePublicationAlreadyPublished,
			Idempotent:                true,
		}, nil
	}

	envelopeID := uuid.NewString()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO envelopes (
		    envelope_id,
		    sender_device_id,
		    recipient_device_id,
		    content_type,
		    protocol_version,
		    ciphertext_b64,
		    payload_sha256,
		    payload_size_bytes,
		    client_created_at,
		    server_received_at,
		    delivery_state,
		    relay_space_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'queued', ?)`,
		envelopeID,
		input.SenderDeviceID,
		input.RecipientDeviceID,
		input.ContentType,
		input.ProtocolVersion,
		input.CiphertextB64,
		input.PayloadSHA256,
		input.PayloadSizeBytes,
		input.ClientCreatedAt,
		input.ServerReceivedAt,
		input.RelaySpaceID,
	)
	if err != nil {
		if isSQLiteBusyError(err) || isSQLiteUniqueError(err) {
			return PublishRelaySpaceKeyPackageResult{},
				errKeyPackagePublicationRetry
		}
		return PublishRelaySpaceKeyPackageResult{},
			fmt.Errorf("insert keypackage publication envelope: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO keypackage_publications (
		    envelope_id,
		    sender_device_id,
		    key_package_ref,
		    payload_sha256,
		    relay_space_id,
		    recipient_device_id,
		    created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		envelopeID,
		input.SenderDeviceID,
		input.KeyPackageRef,
		input.PayloadSHA256,
		input.RelaySpaceID,
		input.RecipientDeviceID,
		input.ServerReceivedAt,
	)
	if err != nil {
		if isSQLiteBusyError(err) || isSQLiteUniqueError(err) {
			return PublishRelaySpaceKeyPackageResult{},
				errKeyPackagePublicationRetry
		}
		return PublishRelaySpaceKeyPackageResult{},
			fmt.Errorf("insert keypackage publication binding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if isSQLiteBusyError(err) || isSQLiteUniqueError(err) {
			return PublishRelaySpaceKeyPackageResult{},
				errKeyPackagePublicationRetry
		}
		return PublishRelaySpaceKeyPackageResult{},
			fmt.Errorf("commit keypackage publication: %w", err)
	}
	committed = true

	return PublishRelaySpaceKeyPackageResult{
		Publication: KeyPackagePublication{
			EnvelopeID:        envelopeID,
			SenderDeviceID:    input.SenderDeviceID,
			KeyPackageRef:     input.KeyPackageRef,
			PayloadSHA256:     input.PayloadSHA256,
			RelaySpaceID:      input.RelaySpaceID,
			RecipientDeviceID: input.RecipientDeviceID,
			CreatedAt:         input.ServerReceivedAt,
			DeliveryState:     "queued",
			ServerReceivedAt:  input.ServerReceivedAt,
			PayloadSizeBytes:  input.PayloadSizeBytes,
		},
		PublicationClassification: KeyPackagePublicationCreated,
		Idempotent:                false,
	}, nil
}

func requireKeyPackagePublicationRelaySpaceTx(
	ctx context.Context,
	tx *sql.Tx,
	relaySpaceID string,
) error {
	var found string
	err := tx.QueryRowContext(
		ctx,
		"SELECT relay_space_id FROM relay_spaces WHERE relay_space_id = ? LIMIT 1",
		relaySpaceID,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrKeyPackagePublicationRelaySpaceNotFound
	}
	if err != nil {
		if isSQLiteBusyError(err) {
			return errKeyPackagePublicationRetry
		}
		return fmt.Errorf("lookup keypackage publication relay space: %w", err)
	}
	return nil
}

func requireKeyPackagePublicationDeviceTx(
	ctx context.Context,
	tx *sql.Tx,
	deviceID string,
	notFound error,
) error {
	var found string
	err := tx.QueryRowContext(
		ctx,
		"SELECT device_id FROM devices WHERE device_id = ? LIMIT 1",
		deviceID,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound
	}
	if err != nil {
		if isSQLiteBusyError(err) {
			return errKeyPackagePublicationRetry
		}
		return fmt.Errorf("lookup keypackage publication device: %w", err)
	}
	return nil
}

func requireActiveKeyPackagePublicationMemberTx(
	ctx context.Context,
	tx *sql.Tx,
	relaySpaceID string,
	deviceID string,
	notMember error,
) error {
	var found int
	err := tx.QueryRowContext(
		ctx,
		`SELECT 1
		   FROM relay_space_members
		  WHERE relay_space_id = ?
		    AND device_id = ?
		    AND state = ?
		    AND disabled_at IS NULL
		  LIMIT 1`,
		relaySpaceID,
		deviceID,
		RelaySpaceMemberStateActive,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return notMember
	}
	if err != nil {
		if isSQLiteBusyError(err) {
			return errKeyPackagePublicationRetry
		}
		return fmt.Errorf("lookup active keypackage publication member: %w", err)
	}
	return nil
}

func keyPackagePublicationByRefTx(
	ctx context.Context,
	tx *sql.Tx,
	senderDeviceID string,
	keyPackageRef string,
) (*KeyPackagePublication, error) {
	return keyPackagePublicationQueryTx(
		ctx,
		tx,
		`SELECT
		    p.envelope_id,
		    p.sender_device_id,
		    p.key_package_ref,
		    p.payload_sha256,
		    p.relay_space_id,
		    p.recipient_device_id,
		    p.created_at,
		    e.delivery_state,
		    e.server_received_at,
		    e.payload_size_bytes
		   FROM keypackage_publications p
		   JOIN envelopes e ON e.envelope_id = p.envelope_id
		  WHERE p.sender_device_id = ?
		    AND p.key_package_ref = ?
		  LIMIT 1`,
		senderDeviceID,
		keyPackageRef,
	)
}

func keyPackagePublicationByPayloadTx(
	ctx context.Context,
	tx *sql.Tx,
	senderDeviceID string,
	payloadSHA256 string,
) (*KeyPackagePublication, error) {
	return keyPackagePublicationQueryTx(
		ctx,
		tx,
		`SELECT
		    p.envelope_id,
		    p.sender_device_id,
		    p.key_package_ref,
		    p.payload_sha256,
		    p.relay_space_id,
		    p.recipient_device_id,
		    p.created_at,
		    e.delivery_state,
		    e.server_received_at,
		    e.payload_size_bytes
		   FROM keypackage_publications p
		   JOIN envelopes e ON e.envelope_id = p.envelope_id
		  WHERE p.sender_device_id = ?
		    AND p.payload_sha256 = ?
		  LIMIT 1`,
		senderDeviceID,
		payloadSHA256,
	)
}

func keyPackagePublicationQueryTx(
	ctx context.Context,
	tx *sql.Tx,
	query string,
	args ...any,
) (*KeyPackagePublication, error) {
	var publication KeyPackagePublication
	err := tx.QueryRowContext(ctx, query, args...).Scan(
		&publication.EnvelopeID,
		&publication.SenderDeviceID,
		&publication.KeyPackageRef,
		&publication.PayloadSHA256,
		&publication.RelaySpaceID,
		&publication.RecipientDeviceID,
		&publication.CreatedAt,
		&publication.DeliveryState,
		&publication.ServerReceivedAt,
		&publication.PayloadSizeBytes,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		if isSQLiteBusyError(err) {
			return nil, errKeyPackagePublicationRetry
		}
		return nil, fmt.Errorf("read keypackage publication: %w", err)
	}
	return &publication, nil
}
