package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/config"
)

func TestCypherConfigInspectionReportsEnvSources(t *testing.T) {
	t.Setenv("CYPHER_ADDR", "127.0.0.1:19080")
	t.Setenv("CYPHER_DB", filepath.Join(t.TempDir(), "cypher.db"))
	t.Setenv("CYPHER_MIGRATIONS", filepath.Join(t.TempDir(), "migrations"))
	t.Setenv("CYPHER_DEV_INVITE", "inspection-test-invite")

	cfg := config.Load()
	report := inspectCypherConfig(cfg)

	if report.SchemaVersion != cypherConfigInspectionSchema {
		t.Fatalf("schema = %q", report.SchemaVersion)
	}
	if report.AddrSource != "env" || report.DBPathSource != "env" || report.MigrationsDirSource != "env" || report.DevInviteSource != "env" {
		t.Fatalf("expected env sources, got %+v", report)
	}
	if report.StartsServer {
		t.Fatal("config inspection must not mark starts_server=true")
	}
	if !report.TerminatingInspection {
		t.Fatal("config inspection must be terminating")
	}
	if report.DBPathIsRepoRelativeDefault {
		t.Fatal("explicit DB path must not be classified as repo-relative default")
	}
}

func TestPrintCypherConfigJSON(t *testing.T) {
	t.Setenv("CYPHER_ADDR", "127.0.0.1:19081")
	t.Setenv("CYPHER_DB", filepath.Join(t.TempDir(), "cypher.db"))
	t.Setenv("CYPHER_MIGRATIONS", filepath.Join(t.TempDir(), "migrations"))

	var buf bytes.Buffer
	if err := printCypherConfig(&buf, config.Load()); err != nil {
		t.Fatalf("print config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	if parsed["schema_version"] != cypherConfigInspectionSchema {
		t.Fatalf("schema mismatch: %+v", parsed)
	}
	if parsed["starts_server"] != false {
		t.Fatalf("print-config must not start server: %+v", parsed)
	}
}

func TestCheckCypherConfigAcceptsExplicitReadableConfig(t *testing.T) {
	tmp := t.TempDir()
	migrations := filepath.Join(tmp, "migrations")
	if err := mkdirAndWriteMigration(migrations); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Addr:          "127.0.0.1:19082",
		DBPath:        filepath.Join(tmp, "state", "cypher.db"),
		MigrationsDir: migrations,
		DevInviteCode: "inspection-test-invite",
	}
	if err := mkdirAll(filepath.Dir(cfg.DBPath)); err != nil {
		t.Fatal(err)
	}
	if err := checkCypherConfig(cfg); err != nil {
		t.Fatalf("check config rejected explicit config: %v", err)
	}
}

func TestCheckCypherConfigRejectsMissingMigrations(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Config{
		Addr:          "127.0.0.1:19083",
		DBPath:        filepath.Join(tmp, "cypher.db"),
		MigrationsDir: filepath.Join(tmp, "missing-migrations"),
		DevInviteCode: "inspection-test-invite",
	}
	if err := checkCypherConfig(cfg); err == nil {
		t.Fatal("expected missing migrations to be rejected")
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o700)
}

func mkdirAndWriteMigration(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "001_test.sql"), []byte("select 1;\n"), 0o600)
}
