package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateRecordsAppliedMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cypher.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open temp DB: %v", err)
	}
	defer store.Close()

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := store.Migrate(migrationsDir); err != nil {
		t.Fatalf("migrate fresh DB: %v", err)
	}

	rows, err := store.DB.Query("SELECT migration_name, sha256, applied_at FROM schema_migrations ORDER BY migration_name ASC")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		var checksum string
		var appliedAt string
		if err := rows.Scan(&name, &checksum, &appliedAt); err != nil {
			t.Fatalf("scan schema_migrations row: %v", err)
		}
		if checksum == "" {
			t.Fatalf("migration %s has empty checksum", name)
		}
		if appliedAt == "" {
			t.Fatalf("migration %s has empty applied_at", name)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}

	want := []string{
		"001_init.sql",
		"002_envelope_payload_metadata.sql",
	}
	if len(names) != len(want) {
		t.Fatalf("migration count = %d, want %d; names=%v", len(names), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("migration[%d] = %q, want %q; all=%v", i, names[i], want[i], names)
		}
	}
}

func TestMigrateCanRunTwiceAgainstSameDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cypher.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open temp DB: %v", err)
	}
	defer store.Close()

	migrationsDir := filepath.Join("..", "..", "migrations")

	if err := store.Migrate(migrationsDir); err != nil {
		t.Fatalf("first migration pass: %v", err)
	}

	if err := store.Migrate(migrationsDir); err != nil {
		t.Fatalf("second migration pass should skip already-applied migrations: %v", err)
	}
}

func TestMigrateDetectsAppliedMigrationChecksumMismatch(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cypher.db")
	migrationsCopy := filepath.Join(tmp, "migrations")

	if err := os.MkdirAll(migrationsCopy, 0o755); err != nil {
		t.Fatalf("create migration copy dir: %v", err)
	}

	sourceMigrations := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(sourceMigrations)
	if err != nil {
		t.Fatalf("read source migrations: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		src := filepath.Join(sourceMigrations, entry.Name())
		dst := filepath.Join(migrationsCopy, entry.Name())

		content, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read source migration %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			t.Fatalf("write copied migration %s: %v", entry.Name(), err)
		}
	}

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open temp DB: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(migrationsCopy); err != nil {
		t.Fatalf("initial migration pass: %v", err)
	}

	migrationToMutate := filepath.Join(migrationsCopy, "002_envelope_payload_metadata.sql")
	f, err := os.OpenFile(migrationToMutate, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open migration for mutation: %v", err)
	}
	if _, err := f.WriteString("\n-- checksum mismatch recon mutation\n"); err != nil {
		_ = f.Close()
		t.Fatalf("mutate migration: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close mutated migration: %v", err)
	}

	err = store.Migrate(migrationsCopy)
	if err == nil {
		t.Fatalf("expected checksum mismatch after mutating an already-applied migration")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
	}
}
