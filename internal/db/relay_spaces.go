package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	RelaySpaceMemberStateActive   = "active"
	RelaySpaceMemberStateDisabled = "disabled"
	RelaySpaceMemberStateLeft     = "left"

	RelaySpaceInviteStateActive   = "active"
	RelaySpaceInviteStateClaimed  = "claimed"
	RelaySpaceInviteStateDisabled = "disabled"
	RelaySpaceInviteStateExpired  = "expired"
)

var (
	ErrRelaySpaceNotFound                 = errors.New("relay space not found")
	ErrRelaySpaceInviteNotFound           = errors.New("relay space invite not found")
	ErrRelaySpaceMemberNotFound           = errors.New("relay space member not found")
	ErrRelaySpaceAccountRequiredForDevice = errors.New("relay space account is required when device is supplied")
	ErrRelaySpaceAccountNotFound          = errors.New("relay space account not found")
	ErrRelaySpaceDeviceNotFound           = errors.New("relay space device not found")
	ErrRelaySpaceAccountDeviceMismatch    = errors.New("relay space account and device do not match")
	ErrRelaySpaceInviteCreatorNotFound    = errors.New("relay space invite creator member not found")
	ErrRelaySpaceInviteCreatorWrongSpace  = errors.New("relay space invite creator belongs to another relay space")
	ErrRelaySpaceInviteCreatorInactive    = errors.New("relay space invite creator member is not active")
)

type RelaySpace struct {
	RelaySpaceID       string `json:"relay_space_id"`
	DisplayLabel       string `json:"display_label"`
	CreatedByAccountID string `json:"created_by_account_id"`
	CreatedByDeviceID  string `json:"created_by_device_id"`
	CreatedAt          string `json:"created_at"`
	DisabledAt         string `json:"disabled_at"`
}

type CreateRelaySpaceInput struct {
	RelaySpaceID       string
	DisplayLabel       string
	CreatedByAccountID string
	CreatedByDeviceID  string
	CreatedAt          string
}

type RelaySpaceInvite struct {
	RelaySpaceInviteID string `json:"relay_space_invite_id"`
	RelaySpaceID       string `json:"relay_space_id"`
	InviteTokenHash    string `json:"invite_token_hash"`
	DisplayCode        string `json:"display_code"`
	WordCode           string `json:"word_code"`
	CreatedByMemberID  string `json:"created_by_member_id"`
	CreatedAt          string `json:"created_at"`
	ExpiresAt          string `json:"expires_at"`
	MaxClaims          *int   `json:"max_claims,omitempty"`
	ClaimCount         int    `json:"claim_count"`
	State              string `json:"state"`
	Note               string `json:"note"`
}

type CreateRelaySpaceInviteInput struct {
	RelaySpaceInviteID string
	RelaySpaceID       string
	InviteToken        string
	InviteTokenHash    string
	DisplayCode        string
	WordCode           string
	CreatedByMemberID  string
	CreatedAt          string
	ExpiresAt          string
	MaxClaims          *int
	State              string
	Note               string
}

type RelaySpaceMember struct {
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

type RegisterRelaySpaceMemberInput struct {
	RoutingMemberID string
	RelaySpaceID    string
	AccountID       string
	DeviceID        string
	DisplayLabel    string
	State           string
	JoinedAt        string
	LastSeenAt      string
}

func (s *Store) validateRelaySpaceAccountDevice(accountID string, deviceID string) error {
	accountID = strings.TrimSpace(accountID)
	deviceID = strings.TrimSpace(deviceID)

	if deviceID != "" && accountID == "" {
		return ErrRelaySpaceAccountRequiredForDevice
	}
	if accountID == "" {
		return nil
	}

	var storedAccountID string
	err := s.DB.QueryRow(
		"SELECT account_id FROM accounts WHERE account_id = ? LIMIT 1",
		accountID,
	).Scan(&storedAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRelaySpaceAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup relay space account: %w", err)
	}

	if deviceID == "" {
		return nil
	}

	var deviceAccountID string
	err = s.DB.QueryRow(
		"SELECT account_id FROM devices WHERE device_id = ? LIMIT 1",
		deviceID,
	).Scan(&deviceAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRelaySpaceDeviceNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup relay space device: %w", err)
	}
	if deviceAccountID != accountID {
		return ErrRelaySpaceAccountDeviceMismatch
	}

	return nil
}

func (s *Store) validateRelaySpaceInviteCreator(
	relaySpaceID string,
	createdByMemberID string,
) error {
	createdByMemberID = strings.TrimSpace(createdByMemberID)
	if createdByMemberID == "" {
		return nil
	}

	member, err := s.GetRelaySpaceMember(createdByMemberID)
	if errors.Is(err, ErrRelaySpaceMemberNotFound) {
		return ErrRelaySpaceInviteCreatorNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup relay space invite creator: %w", err)
	}
	if member.RelaySpaceID != relaySpaceID {
		return ErrRelaySpaceInviteCreatorWrongSpace
	}
	if member.State != RelaySpaceMemberStateActive || member.DisabledAt != "" {
		return ErrRelaySpaceInviteCreatorInactive
	}

	return nil
}

func (s *Store) CreateRelaySpace(input CreateRelaySpaceInput) (RelaySpace, error) {
	input.RelaySpaceID = strings.TrimSpace(input.RelaySpaceID)
	input.DisplayLabel = strings.TrimSpace(input.DisplayLabel)
	input.CreatedByAccountID = strings.TrimSpace(input.CreatedByAccountID)
	input.CreatedByDeviceID = strings.TrimSpace(input.CreatedByDeviceID)
	input.CreatedAt = strings.TrimSpace(input.CreatedAt)

	if input.RelaySpaceID == "" {
		input.RelaySpaceID = uuid.NewString()
	}
	if input.CreatedAt == "" {
		input.CreatedAt = NowUTC()
	}

	if err := s.validateRelaySpaceAccountDevice(
		input.CreatedByAccountID,
		input.CreatedByDeviceID,
	); err != nil {
		return RelaySpace{}, err
	}

	_, err := s.DB.Exec(
		"INSERT INTO relay_spaces (relay_space_id, display_label, created_by_account_id, created_by_device_id, created_at) VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)",
		input.RelaySpaceID,
		input.DisplayLabel,
		input.CreatedByAccountID,
		input.CreatedByDeviceID,
		input.CreatedAt,
	)
	if err != nil {
		return RelaySpace{}, fmt.Errorf("create relay space: %w", err)
	}

	return s.GetRelaySpace(input.RelaySpaceID)
}

func (s *Store) GetRelaySpace(relaySpaceID string) (RelaySpace, error) {
	relaySpaceID = strings.TrimSpace(relaySpaceID)
	if relaySpaceID == "" {
		return RelaySpace{}, ErrRelaySpaceNotFound
	}

	var space RelaySpace
	var createdByAccountID sql.NullString
	var createdByDeviceID sql.NullString
	var disabledAt sql.NullString

	err := s.DB.QueryRow(
		"SELECT relay_space_id, display_label, created_by_account_id, created_by_device_id, created_at, disabled_at FROM relay_spaces WHERE relay_space_id = ? LIMIT 1",
		relaySpaceID,
	).Scan(
		&space.RelaySpaceID,
		&space.DisplayLabel,
		&createdByAccountID,
		&createdByDeviceID,
		&space.CreatedAt,
		&disabledAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return RelaySpace{}, ErrRelaySpaceNotFound
	}
	if err != nil {
		return RelaySpace{}, fmt.Errorf("get relay space: %w", err)
	}

	space.CreatedByAccountID = nullStringValue(createdByAccountID)
	space.CreatedByDeviceID = nullStringValue(createdByDeviceID)
	space.DisabledAt = nullStringValue(disabledAt)

	return space, nil
}

func (s *Store) ListRelaySpaces() ([]RelaySpace, error) {
	rows, err := s.DB.Query(
		"SELECT relay_space_id, display_label, created_by_account_id, created_by_device_id, created_at, disabled_at FROM relay_spaces ORDER BY created_at ASC, relay_space_id ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list relay spaces: %w", err)
	}
	defer rows.Close()

	spaces := []RelaySpace{}
	for rows.Next() {
		var space RelaySpace
		var createdByAccountID sql.NullString
		var createdByDeviceID sql.NullString
		var disabledAt sql.NullString

		if err := rows.Scan(
			&space.RelaySpaceID,
			&space.DisplayLabel,
			&createdByAccountID,
			&createdByDeviceID,
			&space.CreatedAt,
			&disabledAt,
		); err != nil {
			return nil, fmt.Errorf("scan relay space: %w", err)
		}

		space.CreatedByAccountID = nullStringValue(createdByAccountID)
		space.CreatedByDeviceID = nullStringValue(createdByDeviceID)
		space.DisabledAt = nullStringValue(disabledAt)
		spaces = append(spaces, space)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relay spaces: %w", err)
	}

	return spaces, nil
}

func (s *Store) CreateRelaySpaceInvite(input CreateRelaySpaceInviteInput) (RelaySpaceInvite, error) {
	input.RelaySpaceInviteID = strings.TrimSpace(input.RelaySpaceInviteID)
	input.RelaySpaceID = strings.TrimSpace(input.RelaySpaceID)
	input.InviteToken = strings.TrimSpace(input.InviteToken)
	input.InviteTokenHash = strings.TrimSpace(input.InviteTokenHash)
	input.DisplayCode = strings.TrimSpace(input.DisplayCode)
	input.WordCode = strings.TrimSpace(input.WordCode)
	input.CreatedByMemberID = strings.TrimSpace(input.CreatedByMemberID)
	input.CreatedAt = strings.TrimSpace(input.CreatedAt)
	input.ExpiresAt = strings.TrimSpace(input.ExpiresAt)
	input.State = strings.TrimSpace(input.State)
	input.Note = strings.TrimSpace(input.Note)

	if input.RelaySpaceInviteID == "" {
		input.RelaySpaceInviteID = uuid.NewString()
	}
	if input.RelaySpaceID == "" {
		return RelaySpaceInvite{}, errors.New("relay_space_id is required")
	}
	if input.InviteTokenHash == "" {
		if input.InviteToken == "" {
			return RelaySpaceInvite{}, errors.New("invite_token or invite_token_hash is required")
		}
		input.InviteTokenHash = HashInviteCode(input.InviteToken)
	}
	if input.DisplayCode == "" {
		return RelaySpaceInvite{}, errors.New("display_code is required")
	}
	if input.CreatedAt == "" {
		input.CreatedAt = NowUTC()
	}
	if input.State == "" {
		input.State = RelaySpaceInviteStateActive
	}

	if err := s.validateRelaySpaceInviteCreator(
		input.RelaySpaceID,
		input.CreatedByMemberID,
	); err != nil {
		return RelaySpaceInvite{}, err
	}

	_, err := s.DB.Exec(
		"INSERT INTO relay_space_invites (relay_space_invite_id, relay_space_id, invite_token_hash, display_code, word_code, created_by_member_id, created_at, expires_at, max_claims, state, note) VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''))",
		input.RelaySpaceInviteID,
		input.RelaySpaceID,
		input.InviteTokenHash,
		input.DisplayCode,
		input.WordCode,
		input.CreatedByMemberID,
		input.CreatedAt,
		input.ExpiresAt,
		input.MaxClaims,
		input.State,
		input.Note,
	)
	if err != nil {
		return RelaySpaceInvite{}, fmt.Errorf("create relay space invite: %w", err)
	}

	return s.GetRelaySpaceInviteByTokenHash(input.InviteTokenHash)
}

func (s *Store) GetRelaySpaceInviteByTokenHash(inviteTokenHash string) (RelaySpaceInvite, error) {
	inviteTokenHash = strings.TrimSpace(inviteTokenHash)
	if inviteTokenHash == "" {
		return RelaySpaceInvite{}, ErrRelaySpaceInviteNotFound
	}

	var invite RelaySpaceInvite
	var wordCode sql.NullString
	var createdByMemberID sql.NullString
	var expiresAt sql.NullString
	var maxClaims sql.NullInt64
	var note sql.NullString

	err := s.DB.QueryRow(
		"SELECT relay_space_invite_id, relay_space_id, invite_token_hash, display_code, word_code, created_by_member_id, created_at, expires_at, max_claims, claim_count, state, note FROM relay_space_invites WHERE invite_token_hash = ? LIMIT 1",
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
		return RelaySpaceInvite{}, fmt.Errorf("get relay space invite by token hash: %w", err)
	}

	invite.WordCode = nullStringValue(wordCode)
	invite.CreatedByMemberID = nullStringValue(createdByMemberID)
	invite.ExpiresAt = nullStringValue(expiresAt)
	invite.MaxClaims = nullIntValue(maxClaims)
	invite.Note = nullStringValue(note)

	return invite, nil
}

func (s *Store) RegisterRelaySpaceMember(input RegisterRelaySpaceMemberInput) (RelaySpaceMember, error) {
	input.RoutingMemberID = strings.TrimSpace(input.RoutingMemberID)
	input.RelaySpaceID = strings.TrimSpace(input.RelaySpaceID)
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.DisplayLabel = strings.TrimSpace(input.DisplayLabel)
	input.State = strings.TrimSpace(input.State)
	input.JoinedAt = strings.TrimSpace(input.JoinedAt)
	input.LastSeenAt = strings.TrimSpace(input.LastSeenAt)

	if input.RoutingMemberID == "" {
		input.RoutingMemberID = uuid.NewString()
	}
	if input.RelaySpaceID == "" {
		return RelaySpaceMember{}, errors.New("relay_space_id is required")
	}
	if input.AccountID == "" {
		return RelaySpaceMember{}, errors.New("account_id is required")
	}
	if input.State == "" {
		input.State = RelaySpaceMemberStateActive
	}
	if input.JoinedAt == "" {
		input.JoinedAt = NowUTC()
	}

	if err := s.validateRelaySpaceAccountDevice(
		input.AccountID,
		input.DeviceID,
	); err != nil {
		return RelaySpaceMember{}, err
	}

	_, err := s.DB.Exec(
		"INSERT INTO relay_space_members (routing_member_id, relay_space_id, account_id, device_id, display_label, state, joined_at, last_seen_at) VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''))",
		input.RoutingMemberID,
		input.RelaySpaceID,
		input.AccountID,
		input.DeviceID,
		input.DisplayLabel,
		input.State,
		input.JoinedAt,
		input.LastSeenAt,
	)
	if err != nil {
		return RelaySpaceMember{}, fmt.Errorf("register relay space member: %w", err)
	}

	return s.GetRelaySpaceMember(input.RoutingMemberID)
}

func (s *Store) GetRelaySpaceMember(routingMemberID string) (RelaySpaceMember, error) {
	routingMemberID = strings.TrimSpace(routingMemberID)
	if routingMemberID == "" {
		return RelaySpaceMember{}, ErrRelaySpaceMemberNotFound
	}

	var member RelaySpaceMember
	var deviceID sql.NullString
	var lastSeenAt sql.NullString
	var disabledAt sql.NullString

	err := s.DB.QueryRow(
		"SELECT routing_member_id, relay_space_id, account_id, device_id, display_label, state, joined_at, last_seen_at, disabled_at FROM relay_space_members WHERE routing_member_id = ? LIMIT 1",
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
		return RelaySpaceMember{}, fmt.Errorf("get relay space member: %w", err)
	}

	member.DeviceID = nullStringValue(deviceID)
	member.LastSeenAt = nullStringValue(lastSeenAt)
	member.DisabledAt = nullStringValue(disabledAt)

	return member, nil
}

func (s *Store) IsActiveRelaySpaceDeviceMember(relaySpaceID string, deviceID string) (bool, error) {
	relaySpaceID = strings.TrimSpace(relaySpaceID)
	deviceID = strings.TrimSpace(deviceID)

	if relaySpaceID == "" {
		return false, errors.New("relay_space_id is required")
	}
	if deviceID == "" {
		return false, errors.New("device_id is required")
	}

	var found int
	err := s.DB.QueryRow(
		"SELECT 1 FROM relay_space_members WHERE relay_space_id = ? AND device_id = ? AND state = ? AND disabled_at IS NULL LIMIT 1",
		relaySpaceID,
		deviceID,
		RelaySpaceMemberStateActive,
	).Scan(&found)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lookup active relay space device member: %w", err)
	}

	return true, nil
}

func (s *Store) ListRelaySpaceMembers(relaySpaceID string) ([]RelaySpaceMember, error) {
	relaySpaceID = strings.TrimSpace(relaySpaceID)
	if relaySpaceID == "" {
		return nil, errors.New("relay_space_id is required")
	}

	rows, err := s.DB.Query(
		"SELECT routing_member_id, relay_space_id, account_id, device_id, display_label, state, joined_at, last_seen_at, disabled_at FROM relay_space_members WHERE relay_space_id = ? ORDER BY joined_at ASC, routing_member_id ASC",
		relaySpaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list relay space members: %w", err)
	}
	defer rows.Close()

	members := []RelaySpaceMember{}
	for rows.Next() {
		var member RelaySpaceMember
		var deviceID sql.NullString
		var lastSeenAt sql.NullString
		var disabledAt sql.NullString

		if err := rows.Scan(
			&member.RoutingMemberID,
			&member.RelaySpaceID,
			&member.AccountID,
			&deviceID,
			&member.DisplayLabel,
			&member.State,
			&member.JoinedAt,
			&lastSeenAt,
			&disabledAt,
		); err != nil {
			return nil, fmt.Errorf("scan relay space member: %w", err)
		}

		member.DeviceID = nullStringValue(deviceID)
		member.LastSeenAt = nullStringValue(lastSeenAt)
		member.DisabledAt = nullStringValue(disabledAt)
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relay space members: %w", err)
	}

	return members, nil
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullIntValue(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	intValue := int(value.Int64)
	return &intValue
}
