package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/config"
	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/httpapi"
)

const cypherConfigInspectionSchema = "carbonstack-cypher-config-inspection/v0"

type cypherConfigInspection struct {
	SchemaVersion               string `json:"schema_version"`
	Command                     string `json:"command"`
	Addr                        string `json:"addr"`
	AddrSource                  string `json:"addr_source"`
	DBPath                      string `json:"db_path"`
	DBPathSource                string `json:"db_path_source"`
	DBPathIsRepoRelativeDefault bool   `json:"db_path_is_repo_relative_default"`
	MigrationsDir               string `json:"migrations_dir"`
	MigrationsDirSource         string `json:"migrations_dir_source"`
	DevInviteEnabled            bool   `json:"dev_invite_enabled"`
	DevInviteSource             string `json:"dev_invite_source"`
	ServerEntrypoint            bool   `json:"server_entrypoint"`
	StartsServer                bool   `json:"starts_server"`
	TerminatingInspection       bool   `json:"terminating_inspection"`
	GateEManualExplicitEnvNote  string `json:"gate_e_manual_explicit_env_note"`
}

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "CarbonStackCypher server")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "By default this command starts the blocking Cypher HTTP server.")
		fmt.Fprintln(out, "For terminating operator inspection, use --print-config or --check-config.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		flag.PrintDefaults()
	}

	printConfig := flag.Bool("print-config", false, "print effective Cypher config as JSON and exit without starting the server")
	checkConfig := flag.Bool("check-config", false, "validate effective Cypher config and exit without starting the server")
	flag.Parse()

	cfg := config.Load()

	switch {
	case *printConfig:
		if err := printCypherConfig(os.Stdout, cfg); err != nil {
			log.Fatalf("print config: %v", err)
		}
		return
	case *checkConfig:
		if err := checkCypherConfig(cfg); err != nil {
			log.Fatalf("check config: %v", err)
		}
		fmt.Println("config ok")
		return
	default:
		runCypherServer(cfg)
	}
}

func runCypherServer(cfg config.Config) {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(cfg.MigrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	if err := store.SeedDevInvite(cfg.DevInviteCode); err != nil {
		log.Fatalf("seed dev invite: %v", err)
	}

	api := httpapi.New(store, cfg.DevInviteCode != "")

	log.Printf("CarbonStackCypher listening on %s", cfg.Addr)
	log.Printf("database: %s", cfg.DBPath)
	log.Printf("dev invite enabled: %t", cfg.DevInviteCode != "")

	if err := http.ListenAndServe(cfg.Addr, api.Routes()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func printCypherConfig(w io.Writer, cfg config.Config) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(inspectCypherConfig(cfg))
}

func inspectCypherConfig(cfg config.Config) cypherConfigInspection {
	return cypherConfigInspection{
		SchemaVersion:               cypherConfigInspectionSchema,
		Command:                     "cypher-config-inspection",
		Addr:                        cfg.Addr,
		AddrSource:                  envSource("CYPHER_ADDR"),
		DBPath:                      cfg.DBPath,
		DBPathSource:                envSource("CYPHER_DB"),
		DBPathIsRepoRelativeDefault: cfg.DBPath == "cypher.db",
		MigrationsDir:               cfg.MigrationsDir,
		MigrationsDirSource:         envSource("CYPHER_MIGRATIONS"),
		DevInviteEnabled:            cfg.DevInviteCode != "",
		DevInviteSource:             envSource("CYPHER_DEV_INVITE"),
		ServerEntrypoint:            true,
		StartsServer:                false,
		TerminatingInspection:       true,
		GateEManualExplicitEnvNote:  "Gate E deployment should provide explicit CYPHER_ADDR, CYPHER_DB, CYPHER_MIGRATIONS, and CYPHER_DEV_INVITE; do not rely on repo-root defaults for operator deployment.",
	}
}

func envSource(key string) string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return "default"
	}
	return "env"
}

func checkCypherConfig(cfg config.Config) error {
	if strings.TrimSpace(cfg.Addr) == "" {
		return errors.New("CYPHER_ADDR resolved to empty address")
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.Addr); err != nil {
		return fmt.Errorf("CYPHER_ADDR invalid: %w", err)
	}

	if strings.TrimSpace(cfg.DBPath) == "" {
		return errors.New("CYPHER_DB resolved to empty path")
	}

	dbParent := filepath.Dir(cfg.DBPath)
	if dbParent == "" {
		dbParent = "."
	}
	info, err := os.Stat(dbParent)
	if err != nil {
		return fmt.Errorf("CYPHER_DB parent not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("CYPHER_DB parent is not a directory: %s", dbParent)
	}

	if strings.TrimSpace(cfg.MigrationsDir) == "" {
		return errors.New("CYPHER_MIGRATIONS resolved to empty path")
	}
	entries, err := os.ReadDir(cfg.MigrationsDir)
	if err != nil {
		return fmt.Errorf("CYPHER_MIGRATIONS not readable: %w", err)
	}
	hasSQL := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			hasSQL = true
			break
		}
	}
	if !hasSQL {
		return fmt.Errorf("CYPHER_MIGRATIONS has no .sql migrations: %s", cfg.MigrationsDir)
	}

	return nil
}
