package main

import (
	"log"
	"net/http"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/config"
	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/httpapi"
)

func main() {
	cfg := config.Load()

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

	api := httpapi.New(store)

	log.Printf("CarbonStackCypher listening on %s", cfg.Addr)
	log.Printf("database: %s", cfg.DBPath)
	log.Printf("dev invite enabled: %t", cfg.DevInviteCode != "")

	if err := http.ListenAndServe(cfg.Addr, api.Routes()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
