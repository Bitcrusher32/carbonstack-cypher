package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(path string) (*Store, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		conn.Close()
		return nil, err
	}

	return &Store{DB: conn}, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) Migrate(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	sort.Strings(files)

	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		if _, err := s.DB.Exec(string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
	}

	return nil
}

func (s *Store) SeedDevInvite(inviteCode string) error {
	if inviteCode == "" {
		return nil
	}

	hash := HashInviteCode(inviteCode)

	var existing string
	err := s.DB.QueryRow("SELECT invite_id FROM invites WHERE invite_code_hash = ? LIMIT 1", hash).Scan(&existing)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	_, err = s.DB.Exec(
		"INSERT INTO invites (invite_id, invite_code_hash, created_at) VALUES (?, ?, ?)",
		uuid.NewString(),
		hash,
		NowUTC(),
	)
	return err
}

func HashInviteCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func NowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
