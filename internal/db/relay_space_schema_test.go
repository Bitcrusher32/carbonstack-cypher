package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMigrateCreatesRelaySpaceSchema(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	for _, table := range []string{
		"relay_spaces",
		"relay_space_members",
		"relay_space_invites",
	} {
		if !relaySpaceTestTableExists(t, store, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	if !relaySpaceTestColumnExists(t, store, "envelopes", "relay_space_id") {
		t.Fatal("expected envelopes.relay_space_id column to exist")
	}
}

func TestRelaySpaceSchemaAcceptsRoutingOnlyRecords(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := store.DB.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		"account-1",
		"Alice",
		now,
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
		now,
	)
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO relay_spaces (relay_space_id, display_label, created_by_account_id, created_by_device_id, created_at) VALUES (?, ?, ?, ?, ?)",
		"relay-space-1",
		"test relay space",
		"account-1",
		"device-1",
		now,
	)
	if err != nil {
		t.Fatalf("insert relay space: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO relay_space_members (routing_member_id, relay_space_id, account_id, device_id, display_label, state, joined_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"routing-member-1",
		"relay-space-1",
		"account-1",
		"device-1",
		"alice routing member",
		"active",
		now,
	)
	if err != nil {
		t.Fatalf("insert relay space member: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO relay_space_invites (relay_space_invite_id, relay_space_id, invite_token_hash, display_code, word_code, created_by_member_id, created_at, max_claims, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"relay-space-invite-1",
		"relay-space-1",
		"hash-relay-space-secret-token",
		"8F3A-C91B-2D44",
		"banana-wall-red-applesauce",
		"routing-member-1",
		now,
		1,
		"active",
	)
	if err != nil {
		t.Fatalf("insert relay space invite: %v", err)
	}

	var memberState string
	err = store.DB.QueryRow(
		"SELECT state FROM relay_space_members WHERE routing_member_id = ?",
		"routing-member-1",
	).Scan(&memberState)
	if err != nil {
		t.Fatalf("query routing member: %v", err)
	}
	if memberState != "active" {
		t.Fatalf("member state = %q, want active", memberState)
	}

	var claimCount int
	err = store.DB.QueryRow(
		"SELECT claim_count FROM relay_space_invites WHERE relay_space_invite_id = ?",
		"relay-space-invite-1",
	).Scan(&claimCount)
	if err != nil {
		t.Fatalf("query invite claim count: %v", err)
	}
	if claimCount != 0 {
		t.Fatalf("claim_count = %d, want 0", claimCount)
	}
}

func TestRelaySpaceSchemaPreservesRoutingOnlyAuthorityBoundary(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	for _, table := range []string{
		"relay_spaces",
		"relay_space_members",
		"relay_space_invites",
	} {
		columns := relaySpaceTestColumnNames(t, store, table)
		for _, column := range columns {
			lower := strings.ToLower(column)
			if strings.Contains(lower, "trust") {
				t.Fatalf("relay space table %s should not contain trust authority column %q", table, column)
			}
			if strings.Contains(lower, "verified") {
				t.Fatalf("relay space table %s should not contain verified authority column %q", table, column)
			}
		}
	}
}

func TestRelaySpaceMemberRejectsInvalidState(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := store.DB.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		"account-1",
		"Alice",
		now,
	)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO relay_spaces (relay_space_id, display_label, created_by_account_id, created_at) VALUES (?, ?, ?, ?)",
		"relay-space-1",
		"test relay space",
		"account-1",
		now,
	)
	if err != nil {
		t.Fatalf("insert relay space: %v", err)
	}

	_, err = store.DB.Exec(
		"INSERT INTO relay_space_members (routing_member_id, relay_space_id, account_id, display_label, state, joined_at) VALUES (?, ?, ?, ?, ?, ?)",
		"routing-member-1",
		"relay-space-1",
		"account-1",
		"alice routing member",
		"verified",
		now,
	)
	if err == nil {
		t.Fatal("expected invalid routing member state to fail")
	}
}

func openMigratedRelaySpaceTestStore(t *testing.T) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cypher.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open temp DB: %v", err)
	}

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := store.Migrate(migrationsDir); err != nil {
		_ = store.Close()
		t.Fatalf("migrate temp DB: %v", err)
	}

	return store
}

func relaySpaceTestTableExists(t *testing.T, store *Store, table string) bool {
	t.Helper()

	var name string
	err := store.DB.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&name)

	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}

	return name == table
}

func relaySpaceTestColumnExists(t *testing.T, store *Store, table string, column string) bool {
	t.Helper()

	for _, candidate := range relaySpaceTestColumnNames(t, store, table) {
		if candidate == column {
			return true
		}
	}

	return false
}

func relaySpaceTestColumnNames(t *testing.T, store *Store, table string) []string {
	t.Helper()

	rows, err := store.DB.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("read table info for %s: %v", table, err)
	}
	defer rows.Close()

	names := []string{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int

		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info for %s: %v", table, err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info for %s: %v", table, err)
	}

	return names
}
