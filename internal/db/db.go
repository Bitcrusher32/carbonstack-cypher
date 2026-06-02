package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	if _, err := s.DB.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    migration_name TEXT PRIMARY KEY,
    sha256 TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(files, func(i int, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file.Name(), err)
		}

		sum := sha256.Sum256(content)
		checksum := hex.EncodeToString(sum[:])

		var appliedChecksum string
		err = s.DB.QueryRow(
			"SELECT sha256 FROM schema_migrations WHERE migration_name = ? LIMIT 1",
			file.Name(),
		).Scan(&appliedChecksum)

		switch {
		case err == nil:
			if appliedChecksum != checksum {
				return fmt.Errorf(
					"migration %s checksum mismatch: applied %s, current %s",
					file.Name(),
					appliedChecksum,
					checksum,
				)
			}
			continue

		case errors.Is(err, sql.ErrNoRows):
			// Apply below.

		case err != nil:
			return fmt.Errorf("read migration state %s: %w", file.Name(), err)
		}

		tx, err := s.DB.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", file.Name(), err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", file.Name(), err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (migration_name, sha256, applied_at) VALUES (?, ?, ?)",
			file.Name(),
			checksum,
			NowUTC(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", file.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", file.Name(), err)
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
