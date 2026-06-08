package db

import (
	"errors"
	"strings"
	"testing"
)

func TestCreateGetAndListRelaySpaces(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)

	created, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-1",
		DisplayLabel:       "  test relay space  ",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
		CreatedAt:          "2026-06-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("create relay space: %v", err)
	}

	if created.RelaySpaceID != "relay-space-1" {
		t.Fatalf("relay_space_id = %q, want relay-space-1", created.RelaySpaceID)
	}
	if created.DisplayLabel != "test relay space" {
		t.Fatalf("display_label = %q, want trimmed label", created.DisplayLabel)
	}
	if created.CreatedByAccountID != "account-1" {
		t.Fatalf("created_by_account_id = %q, want account-1", created.CreatedByAccountID)
	}
	if created.CreatedByDeviceID != "device-1" {
		t.Fatalf("created_by_device_id = %q, want device-1", created.CreatedByDeviceID)
	}
	if created.CreatedAt != "2026-06-08T00:00:00Z" {
		t.Fatalf("created_at = %q", created.CreatedAt)
	}

	got, err := store.GetRelaySpace("relay-space-1")
	if err != nil {
		t.Fatalf("get relay space: %v", err)
	}
	if got != created {
		t.Fatalf("got relay space = %+v, want %+v", got, created)
	}

	spaces, err := store.ListRelaySpaces()
	if err != nil {
		t.Fatalf("list relay spaces: %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("len(spaces) = %d, want 1", len(spaces))
	}
	if spaces[0].RelaySpaceID != "relay-space-1" {
		t.Fatalf("spaces[0].relay_space_id = %q", spaces[0].RelaySpaceID)
	}
}

func TestGetRelaySpaceNotFound(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	_, err := store.GetRelaySpace("missing")
	if !errors.Is(err, ErrRelaySpaceNotFound) {
		t.Fatalf("err = %v, want ErrRelaySpaceNotFound", err)
	}
}

func TestCreateRelaySpaceInviteAndLookupByTokenHash(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	space := createRelaySpaceForHelperTest(t, store)
	member := registerRelaySpaceMemberForHelperTest(t, store, space.RelaySpaceID)

	maxClaims := 2
	created, err := store.CreateRelaySpaceInvite(CreateRelaySpaceInviteInput{
		RelaySpaceInviteID: "invite-1",
		RelaySpaceID:       space.RelaySpaceID,
		InviteToken:        "secret relay space token",
		DisplayCode:        "8F3A-C91B-2D44",
		WordCode:           "banana-wall-red-applesauce",
		CreatedByMemberID:  member.RoutingMemberID,
		CreatedAt:          "2026-06-08T00:00:00Z",
		MaxClaims:          &maxClaims,
		Note:               "  routing-only invite  ",
	})
	if err != nil {
		t.Fatalf("create relay space invite: %v", err)
	}

	wantHash := HashInviteCode("secret relay space token")
	if created.InviteTokenHash != wantHash {
		t.Fatalf("invite_token_hash = %q, want %q", created.InviteTokenHash, wantHash)
	}
	if created.State != RelaySpaceInviteStateActive {
		t.Fatalf("state = %q, want active", created.State)
	}
	if created.ClaimCount != 0 {
		t.Fatalf("claim_count = %d, want 0", created.ClaimCount)
	}
	if created.MaxClaims == nil || *created.MaxClaims != 2 {
		t.Fatalf("max_claims = %v, want 2", created.MaxClaims)
	}
	if created.Note != "routing-only invite" {
		t.Fatalf("note = %q, want trimmed note", created.Note)
	}

	got, err := store.GetRelaySpaceInviteByTokenHash(wantHash)
	if err != nil {
		t.Fatalf("get relay space invite by token hash: %v", err)
	}
	if got.RelaySpaceInviteID != "invite-1" {
		t.Fatalf("relay_space_invite_id = %q, want invite-1", got.RelaySpaceInviteID)
	}
	if got.WordCode != "banana-wall-red-applesauce" {
		t.Fatalf("word_code = %q", got.WordCode)
	}
}

func TestGetRelaySpaceInviteByTokenHashNotFound(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	_, err := store.GetRelaySpaceInviteByTokenHash("missing")
	if !errors.Is(err, ErrRelaySpaceInviteNotFound) {
		t.Fatalf("err = %v, want ErrRelaySpaceInviteNotFound", err)
	}
}

func TestRegisterGetAndListRelaySpaceMembers(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	seedRelaySpaceAccountAndDevice(t, store)
	space := createRelaySpaceForHelperTest(t, store)

	member, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "routing-member-1",
		RelaySpaceID:    space.RelaySpaceID,
		AccountID:       "account-1",
		DeviceID:        "device-1",
		DisplayLabel:    "  alice routing member  ",
		JoinedAt:        "2026-06-08T00:00:00Z",
		LastSeenAt:      "2026-06-08T00:01:00Z",
	})
	if err != nil {
		t.Fatalf("register relay space member: %v", err)
	}

	if member.RoutingMemberID != "routing-member-1" {
		t.Fatalf("routing_member_id = %q", member.RoutingMemberID)
	}
	if member.State != RelaySpaceMemberStateActive {
		t.Fatalf("state = %q, want active", member.State)
	}
	if member.DisplayLabel != "alice routing member" {
		t.Fatalf("display_label = %q, want trimmed label", member.DisplayLabel)
	}
	if member.LastSeenAt != "2026-06-08T00:01:00Z" {
		t.Fatalf("last_seen_at = %q", member.LastSeenAt)
	}

	got, err := store.GetRelaySpaceMember("routing-member-1")
	if err != nil {
		t.Fatalf("get relay space member: %v", err)
	}
	if got != member {
		t.Fatalf("got relay space member = %+v, want %+v", got, member)
	}

	members, err := store.ListRelaySpaceMembers(space.RelaySpaceID)
	if err != nil {
		t.Fatalf("list relay space members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("len(members) = %d, want 1", len(members))
	}
	if members[0].RoutingMemberID != "routing-member-1" {
		t.Fatalf("members[0].routing_member_id = %q", members[0].RoutingMemberID)
	}
}

func TestGetRelaySpaceMemberNotFound(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	_, err := store.GetRelaySpaceMember("missing")
	if !errors.Is(err, ErrRelaySpaceMemberNotFound) {
		t.Fatalf("err = %v, want ErrRelaySpaceMemberNotFound", err)
	}
}

func TestRelaySpaceHelpersPreserveRoutingOnlyNamingBoundary(t *testing.T) {
	names := []string{
		"RelaySpace",
		"RelaySpaceInvite",
		"RelaySpaceMember",
		"RoutingMemberID",
		"CreateRelaySpace",
		"CreateRelaySpaceInvite",
		"RegisterRelaySpaceMember",
		"ListRelaySpaceMembers",
	}

	for _, name := range names {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "verified") {
			t.Fatalf("helper name %q must not imply verified identity", name)
		}
		if strings.Contains(lower, "trust") {
			t.Fatalf("helper name %q must not imply trust authority", name)
		}
	}
}

func seedRelaySpaceAccountAndDevice(t *testing.T, store *Store) {
	t.Helper()

	_, err := store.DB.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		"account-1",
		"Alice",
		"2026-06-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO devices (device_id, account_id, device_label, public_identity_key, public_prekey_bundle, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"device-1",
		"account-1",
		"alice-device",
		"alice-public-identity-key",
		"alice-prekey-bundle",
		"2026-06-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
}

func createRelaySpaceForHelperTest(t *testing.T, store *Store) RelaySpace {
	t.Helper()

	space, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "relay-space-1",
		DisplayLabel:       "test relay space",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
		CreatedAt:          "2026-06-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("create relay space fixture: %v", err)
	}

	return space
}

func registerRelaySpaceMemberForHelperTest(t *testing.T, store *Store, relaySpaceID string) RelaySpaceMember {
	t.Helper()

	member, err := store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "routing-member-1",
		RelaySpaceID:    relaySpaceID,
		AccountID:       "account-1",
		DeviceID:        "device-1",
		DisplayLabel:    "alice routing member",
		JoinedAt:        "2026-06-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("register relay space member fixture: %v", err)
	}

	return member
}
