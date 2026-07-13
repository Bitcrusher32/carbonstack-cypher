package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	RelaySpaceMemberStateTransitioned   = "transitioned"
	RelaySpaceMemberStateAlreadyCurrent = "already_in_state"
)

var (
	ErrRelaySpaceMemberWrongSpace = errors.New(
		"relay space member belongs to another relay space",
	)
	ErrRelaySpaceMemberTargetStateUnsupported = errors.New(
		"relay space member target state is unsupported",
	)
	ErrRelaySpaceMemberStoredStateUnsupported = errors.New(
		"relay space member stored state is unsupported",
	)
	ErrRelaySpaceMemberStateInconsistent = errors.New(
		"relay space member state and disabled_at are inconsistent",
	)
	ErrRelaySpaceMemberRejoinRequired = errors.New(
		"left relay space member requires an explicit rejoin workflow",
	)
	ErrRelaySpaceMemberStateContended = errors.New(
		"relay space member state transition remained contended",
	)
)

var errRelaySpaceMemberStateRetry = errors.New(
	"retry relay space member state transition",
)

type UpdateRelaySpaceMemberStateInput struct {
	RelaySpaceID    string
	RoutingMemberID string
	TargetState     string
	ChangedAt       string
}

type RelaySpaceMemberStateResult struct {
	RoutingMember            RelaySpaceMember `json:"routing_member"`
	PreviousState            string           `json:"previous_state"`
	CurrentState             string           `json:"current_state"`
	TransitionClassification string           `json:"transition_classification"`
	Idempotent               bool             `json:"idempotent"`
	TransitionedAt           string           `json:"transitioned_at,omitempty"`
}

func (s *Store) UpdateRelaySpaceMemberState(
	input UpdateRelaySpaceMemberStateInput,
) (RelaySpaceMemberStateResult, error) {
	input.RelaySpaceID = strings.TrimSpace(input.RelaySpaceID)
	input.RoutingMemberID = strings.TrimSpace(input.RoutingMemberID)
	input.TargetState = strings.TrimSpace(input.TargetState)
	input.ChangedAt = strings.TrimSpace(input.ChangedAt)

	if input.RelaySpaceID == "" {
		return RelaySpaceMemberStateResult{}, errors.New(
			"relay_space_id is required",
		)
	}
	if input.RoutingMemberID == "" {
		return RelaySpaceMemberStateResult{}, errors.New(
			"routing_member_id is required",
		)
	}
	if !isSupportedRelaySpaceMemberTargetState(input.TargetState) {
		return RelaySpaceMemberStateResult{},
			ErrRelaySpaceMemberTargetStateUnsupported
	}
	if input.ChangedAt == "" {
		input.ChangedAt = NowUTC()
	}
	if _, err := time.Parse(time.RFC3339, input.ChangedAt); err != nil {
		return RelaySpaceMemberStateResult{}, fmt.Errorf(
			"parse changed_at: %w",
			err,
		)
	}

	for attempt := 0; attempt < 30; attempt++ {
		result, err := s.updateRelaySpaceMemberStateOnce(
			context.Background(),
			input,
		)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errRelaySpaceMemberStateRetry) &&
			!isSQLiteBusyError(err) {
			return RelaySpaceMemberStateResult{}, err
		}

		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}

	return RelaySpaceMemberStateResult{},
		ErrRelaySpaceMemberStateContended
}

func (s *Store) updateRelaySpaceMemberStateOnce(
	ctx context.Context,
	input UpdateRelaySpaceMemberStateInput,
) (RelaySpaceMemberStateResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		if isSQLiteBusyError(err) {
			return RelaySpaceMemberStateResult{},
				errRelaySpaceMemberStateRetry
		}
		return RelaySpaceMemberStateResult{}, fmt.Errorf(
			"begin relay space member state transition: %w",
			err,
		)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	member, err := getRelaySpaceMemberForStateTx(
		ctx,
		tx,
		input.RoutingMemberID,
	)
	if err != nil {
		return RelaySpaceMemberStateResult{}, err
	}
	if member.RelaySpaceID != input.RelaySpaceID {
		return RelaySpaceMemberStateResult{},
			ErrRelaySpaceMemberWrongSpace
	}
	if err := validateStoredRelaySpaceMemberState(member); err != nil {
		return RelaySpaceMemberStateResult{}, err
	}

	if member.State == input.TargetState {
		_ = tx.Rollback()
		return RelaySpaceMemberStateResult{
			RoutingMember:            member,
			PreviousState:            member.State,
			CurrentState:             member.State,
			TransitionClassification: RelaySpaceMemberStateAlreadyCurrent,
			Idempotent:               true,
		}, nil
	}

	if member.State == RelaySpaceMemberStateLeft {
		return RelaySpaceMemberStateResult{},
			ErrRelaySpaceMemberRejoinRequired
	}

	if !isAllowedRelaySpaceMemberTransition(
		member.State,
		input.TargetState,
	) {
		return RelaySpaceMemberStateResult{},
			ErrRelaySpaceMemberTargetStateUnsupported
	}

	newDisabledAt := ""
	if input.TargetState == RelaySpaceMemberStateDisabled {
		newDisabledAt = input.ChangedAt
	}

	update, err := tx.ExecContext(
		ctx,
		`UPDATE relay_space_members
		    SET state = ?,
		        disabled_at = NULLIF(?, '')
		  WHERE routing_member_id = ?
		    AND relay_space_id = ?
		    AND state = ?
		    AND COALESCE(disabled_at, '') = ?`,
		input.TargetState,
		newDisabledAt,
		member.RoutingMemberID,
		member.RelaySpaceID,
		member.State,
		member.DisabledAt,
	)
	if err != nil {
		if isSQLiteBusyError(err) {
			return RelaySpaceMemberStateResult{},
				errRelaySpaceMemberStateRetry
		}
		return RelaySpaceMemberStateResult{}, fmt.Errorf(
			"update relay space member state: %w",
			err,
		)
	}

	rowsAffected, err := update.RowsAffected()
	if err != nil {
		return RelaySpaceMemberStateResult{}, fmt.Errorf(
			"inspect relay space member state update: %w",
			err,
		)
	}
	if rowsAffected != 1 {
		return RelaySpaceMemberStateResult{},
			errRelaySpaceMemberStateRetry
	}

	if err := tx.Commit(); err != nil {
		if isSQLiteBusyError(err) {
			return RelaySpaceMemberStateResult{},
				errRelaySpaceMemberStateRetry
		}
		return RelaySpaceMemberStateResult{}, fmt.Errorf(
			"commit relay space member state transition: %w",
			err,
		)
	}
	committed = true

	updated, err := s.GetRelaySpaceMember(member.RoutingMemberID)
	if err != nil {
		return RelaySpaceMemberStateResult{}, err
	}

	return RelaySpaceMemberStateResult{
		RoutingMember:            updated,
		PreviousState:            member.State,
		CurrentState:             updated.State,
		TransitionClassification: RelaySpaceMemberStateTransitioned,
		Idempotent:               false,
		TransitionedAt:           input.ChangedAt,
	}, nil
}

func isSupportedRelaySpaceMemberTargetState(state string) bool {
	switch state {
	case RelaySpaceMemberStateActive,
		RelaySpaceMemberStateDisabled,
		RelaySpaceMemberStateLeft:
		return true
	default:
		return false
	}
}

func isAllowedRelaySpaceMemberTransition(
	currentState string,
	targetState string,
) bool {
	switch currentState {
	case RelaySpaceMemberStateActive:
		return targetState == RelaySpaceMemberStateDisabled ||
			targetState == RelaySpaceMemberStateLeft
	case RelaySpaceMemberStateDisabled:
		return targetState == RelaySpaceMemberStateActive ||
			targetState == RelaySpaceMemberStateLeft
	default:
		return false
	}
}

func validateStoredRelaySpaceMemberState(
	member RelaySpaceMember,
) error {
	switch member.State {
	case RelaySpaceMemberStateActive:
		if member.DisabledAt != "" {
			return ErrRelaySpaceMemberStateInconsistent
		}
	case RelaySpaceMemberStateDisabled:
		if member.DisabledAt == "" {
			return ErrRelaySpaceMemberStateInconsistent
		}
	case RelaySpaceMemberStateLeft:
		if member.DisabledAt != "" {
			return ErrRelaySpaceMemberStateInconsistent
		}
	default:
		return ErrRelaySpaceMemberStoredStateUnsupported
	}

	return nil
}

func getRelaySpaceMemberForStateTx(
	ctx context.Context,
	tx *sql.Tx,
	routingMemberID string,
) (RelaySpaceMember, error) {
	var member RelaySpaceMember
	var deviceID sql.NullString
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
		  WHERE routing_member_id = ?
		  LIMIT 1`,
		routingMemberID,
	).Scan(
		&member.RoutingMemberID,
		&member.RelaySpaceID,
		&member.AccountID,
		&deviceID,
		&member.DisplayLabel,
		&member.State,
		&member.JoinedAt,
		&lastSeenAt,
		&disabledAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RelaySpaceMember{}, ErrRelaySpaceMemberNotFound
	}
	if err != nil {
		return RelaySpaceMember{}, fmt.Errorf(
			"get relay space member for state transition: %w",
			err,
		)
	}

	member.DeviceID = deviceID.String
	member.LastSeenAt = lastSeenAt.String
	member.DisabledAt = disabledAt.String

	return member, nil
}
