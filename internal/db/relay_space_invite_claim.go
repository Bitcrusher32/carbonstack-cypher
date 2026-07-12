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
	RelaySpaceInviteClaimCreated       = "created"
	RelaySpaceInviteClaimAlreadyActive = "already_active"
)

var (
	ErrRelaySpaceInviteDisabled         = errors.New("relay space invite is disabled")
	ErrRelaySpaceInviteExpired          = errors.New("relay space invite is expired")
	ErrRelaySpaceInviteExhausted        = errors.New("relay space invite is exhausted")
	ErrRelaySpaceInviteUnsupportedState = errors.New("relay space invite has unsupported state")
	ErrRelaySpaceInviteExpiryInvalid    = errors.New("relay space invite expiry is invalid")
	ErrRelaySpaceInviteClaimContended   = errors.New("relay space invite claim remained contended")
	ErrRelaySpaceMemberAccountConflict  = errors.New("relay space member device belongs to another account")
	ErrRelaySpaceMemberDisabled         = errors.New("relay space member is disabled")
	ErrRelaySpaceMemberLeft             = errors.New("relay space member has left")
)

var errRelaySpaceInviteClaimRetry = errors.New("retry relay space invite claim")

type ClaimRelaySpaceInviteInput struct {
	InviteToken  string
	AccountID    string
	DeviceID     string
	DisplayLabel string
	ClaimedAt    string
}

type RelaySpaceInviteClaimResult struct {
	RelaySpace          RelaySpace       `json:"relay_space"`
	RoutingMember       RelaySpaceMember `json:"routing_member"`
	RelaySpaceInvite    RelaySpaceInvite `json:"relay_space_invite"`
	ClaimClassification string           `json:"claim_classification"`
	Idempotent          bool             `json:"idempotent"`
	ClaimConsumed       bool             `json:"claim_consumed"`
}

func (s *Store) ClaimRelaySpaceInvite(
	input ClaimRelaySpaceInviteInput,
) (RelaySpaceInviteClaimResult, error) {
	input.InviteToken = strings.TrimSpace(input.InviteToken)
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.DisplayLabel = strings.TrimSpace(input.DisplayLabel)
	input.ClaimedAt = strings.TrimSpace(input.ClaimedAt)

	if input.InviteToken == "" {
		return RelaySpaceInviteClaimResult{}, errors.New("invite_token is required")
	}
	if input.AccountID == "" {
		return RelaySpaceInviteClaimResult{}, errors.New("account_id is required")
	}
	if input.DeviceID == "" {
		return RelaySpaceInviteClaimResult{}, errors.New("device_id is required")
	}
	if input.ClaimedAt == "" {
		input.ClaimedAt = NowUTC()
	}

	claimedAt, err := time.Parse(time.RFC3339, input.ClaimedAt)
	if err != nil {
		return RelaySpaceInviteClaimResult{}, fmt.Errorf("parse claimed_at: %w", err)
	}

	if err := s.validateRelaySpaceAccountDevice(input.AccountID, input.DeviceID); err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}

	for attempt := 0; attempt < 30; attempt++ {
		result, err := s.claimRelaySpaceInviteOnce(
			context.Background(),
			input,
			claimedAt,
		)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errRelaySpaceInviteClaimRetry) && !isSQLiteBusyError(err) {
			return RelaySpaceInviteClaimResult{}, err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}

	return RelaySpaceInviteClaimResult{}, ErrRelaySpaceInviteClaimContended
}

func (s *Store) claimRelaySpaceInviteOnce(
	ctx context.Context,
	input ClaimRelaySpaceInviteInput,
	claimedAt time.Time,
) (RelaySpaceInviteClaimResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		if isSQLiteBusyError(err) {
			return RelaySpaceInviteClaimResult{}, errRelaySpaceInviteClaimRetry
		}
		return RelaySpaceInviteClaimResult{}, fmt.Errorf(
			"begin relay space invite claim: %w",
			err,
		)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	inviteTokenHash := HashInviteCode(input.InviteToken)
	invite, err := getRelaySpaceInviteByTokenHashTx(ctx, tx, inviteTokenHash)
	if err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}

	existingMember, found, err := getRelaySpaceMemberByDeviceTx(
		ctx,
		tx,
		invite.RelaySpaceID,
		input.DeviceID,
	)
	if err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}
	if found {
		switch {
		case existingMember.AccountID != input.AccountID:
			return RelaySpaceInviteClaimResult{}, ErrRelaySpaceMemberAccountConflict
		case existingMember.State == RelaySpaceMemberStateDisabled ||
			existingMember.DisabledAt != "":
			return RelaySpaceInviteClaimResult{}, ErrRelaySpaceMemberDisabled
		case existingMember.State == RelaySpaceMemberStateLeft:
			return RelaySpaceInviteClaimResult{}, ErrRelaySpaceMemberLeft
		case existingMember.State != RelaySpaceMemberStateActive:
			return RelaySpaceInviteClaimResult{}, ErrRelaySpaceInviteUnsupportedState
		}

		_ = tx.Rollback()
		return s.assembleRelaySpaceInviteClaimResult(
			inviteTokenHash,
			existingMember.RoutingMemberID,
			RelaySpaceInviteClaimAlreadyActive,
			true,
			false,
		)
	}

	if err := validateRelaySpaceInviteClaimable(invite, claimedAt); err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}

	reservation, err := tx.ExecContext(
		ctx,
		`UPDATE relay_space_invites
		 SET claim_count = claim_count + 1,
		     state = CASE
		         WHEN max_claims IS NOT NULL
		              AND claim_count + 1 >= max_claims
		         THEN ?
		         ELSE state
		     END
		 WHERE relay_space_invite_id = ?
		   AND state = ?
		   AND claim_count = ?
		   AND (max_claims IS NULL OR claim_count < max_claims)`,
		RelaySpaceInviteStateClaimed,
		invite.RelaySpaceInviteID,
		RelaySpaceInviteStateActive,
		invite.ClaimCount,
	)
	if err != nil {
		if isSQLiteBusyError(err) {
			return RelaySpaceInviteClaimResult{}, errRelaySpaceInviteClaimRetry
		}
		return RelaySpaceInviteClaimResult{}, fmt.Errorf(
			"reserve relay space invite claim: %w",
			err,
		)
	}

	rowsAffected, err := reservation.RowsAffected()
	if err != nil {
		return RelaySpaceInviteClaimResult{}, fmt.Errorf(
			"inspect relay space invite reservation: %w",
			err,
		)
	}
	if rowsAffected != 1 {
		return RelaySpaceInviteClaimResult{}, errRelaySpaceInviteClaimRetry
	}

	memberID := uuid.NewString()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO relay_space_members (
		    routing_member_id,
		    relay_space_id,
		    account_id,
		    device_id,
		    display_label,
		    state,
		    joined_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		memberID,
		invite.RelaySpaceID,
		input.AccountID,
		input.DeviceID,
		input.DisplayLabel,
		RelaySpaceMemberStateActive,
		input.ClaimedAt,
	)
	if err != nil {
		if isSQLiteBusyError(err) || isSQLiteUniqueError(err) {
			return RelaySpaceInviteClaimResult{}, errRelaySpaceInviteClaimRetry
		}
		return RelaySpaceInviteClaimResult{}, fmt.Errorf(
			"create relay space member from invite claim: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		if isSQLiteBusyError(err) {
			return RelaySpaceInviteClaimResult{}, errRelaySpaceInviteClaimRetry
		}
		return RelaySpaceInviteClaimResult{}, fmt.Errorf(
			"commit relay space invite claim: %w",
			err,
		)
	}
	committed = true

	return s.assembleRelaySpaceInviteClaimResult(
		inviteTokenHash,
		memberID,
		RelaySpaceInviteClaimCreated,
		false,
		true,
	)
}

func validateRelaySpaceInviteClaimable(
	invite RelaySpaceInvite,
	claimedAt time.Time,
) error {
	switch invite.State {
	case RelaySpaceInviteStateActive:
	case RelaySpaceInviteStateDisabled:
		return ErrRelaySpaceInviteDisabled
	case RelaySpaceInviteStateExpired:
		return ErrRelaySpaceInviteExpired
	case RelaySpaceInviteStateClaimed:
		return ErrRelaySpaceInviteExhausted
	default:
		return ErrRelaySpaceInviteUnsupportedState
	}

	if invite.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, invite.ExpiresAt)
		if err != nil {
			return ErrRelaySpaceInviteExpiryInvalid
		}
		if !claimedAt.Before(expiresAt) {
			return ErrRelaySpaceInviteExpired
		}
	}

	if invite.MaxClaims != nil && invite.ClaimCount >= *invite.MaxClaims {
		return ErrRelaySpaceInviteExhausted
	}

	return nil
}

func (s *Store) assembleRelaySpaceInviteClaimResult(
	inviteTokenHash string,
	routingMemberID string,
	classification string,
	idempotent bool,
	claimConsumed bool,
) (RelaySpaceInviteClaimResult, error) {
	invite, err := s.GetRelaySpaceInviteByTokenHash(inviteTokenHash)
	if err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}
	space, err := s.GetRelaySpace(invite.RelaySpaceID)
	if err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}
	member, err := s.GetRelaySpaceMember(routingMemberID)
	if err != nil {
		return RelaySpaceInviteClaimResult{}, err
	}

	return RelaySpaceInviteClaimResult{
		RelaySpace:          space,
		RoutingMember:       member,
		RelaySpaceInvite:    invite,
		ClaimClassification: classification,
		Idempotent:          idempotent,
		ClaimConsumed:       claimConsumed,
	}, nil
}

func getRelaySpaceInviteByTokenHashTx(
	ctx context.Context,
	tx *sql.Tx,
	inviteTokenHash string,
) (RelaySpaceInvite, error) {
	var invite RelaySpaceInvite
	var wordCode sql.NullString
	var createdByMemberID sql.NullString
	var expiresAt sql.NullString
	var maxClaims sql.NullInt64
	var note sql.NullString

	err := tx.QueryRowContext(
		ctx,
		`SELECT relay_space_invite_id,
		        relay_space_id,
		        invite_token_hash,
		        display_code,
		        word_code,
		        created_by_member_id,
		        created_at,
		        expires_at,
		        max_claims,
		        claim_count,
		        state,
		        note
		   FROM relay_space_invites
		  WHERE invite_token_hash = ?
		  LIMIT 1`,
		inviteTokenHash,
	).Scan(
		&invite.RelaySpaceInviteID,
		&invite.RelaySpaceID,
		&invite.InviteTokenHash,
		&invite.DisplayCode,
		&wordCode,
		&createdByMemberID,
		&invite.CreatedAt,
		&expiresAt,
		&maxClaims,
		&invite.ClaimCount,
		&invite.State,
		&note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RelaySpaceInvite{}, ErrRelaySpaceInviteNotFound
	}
	if err != nil {
		return RelaySpaceInvite{}, fmt.Errorf("get relay space invite for claim: %w", err)
	}

	invite.WordCode = wordCode.String
	invite.CreatedByMemberID = createdByMemberID.String
	invite.ExpiresAt = expiresAt.String
	if maxClaims.Valid {
		value := int(maxClaims.Int64)
		invite.MaxClaims = &value
	}
	invite.Note = note.String

	return invite, nil
}

func getRelaySpaceMemberByDeviceTx(
	ctx context.Context,
	tx *sql.Tx,
	relaySpaceID string,
	deviceID string,
) (RelaySpaceMember, bool, error) {
	var member RelaySpaceMember
	var nullableDeviceID sql.NullString
	var lastSeenAt sql.NullString
	var disabledAt sql.NullString

	err := tx.QueryRowContext(
		ctx,
		`SELECT routing_member_id,
		        relay_space_id,
		        account_id,
		        device_id,
		        display_label,
		        state,
		        joined_at,
		        last_seen_at,
		        disabled_at
		   FROM relay_space_members
		  WHERE relay_space_id = ?
		    AND device_id = ?
		  LIMIT 1`,
		relaySpaceID,
		deviceID,
	).Scan(
		&member.RoutingMemberID,
		&member.RelaySpaceID,
		&member.AccountID,
		&nullableDeviceID,
		&member.DisplayLabel,
		&member.State,
		&member.JoinedAt,
		&lastSeenAt,
		&disabledAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RelaySpaceMember{}, false, nil
	}
	if err != nil {
		return RelaySpaceMember{}, false, fmt.Errorf(
			"get relay space member by device for claim: %w",
			err,
		)
	}

	member.DeviceID = nullableDeviceID.String
	member.LastSeenAt = lastSeenAt.String
	member.DisabledAt = disabledAt.String
	return member, true, nil
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") ||
		strings.Contains(text, "database table is locked") ||
		strings.Contains(text, "sqlite_busy") ||
		strings.Contains(text, "(5)")
}

func isSQLiteUniqueError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unique constraint failed") ||
		strings.Contains(text, "constraint failed")
}
