package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed all:sqlite/migrations
var sqliteMigrations embed.FS

// Migrate runs pending migrations for the given driver.
func Migrate(db *sql.DB, driver string) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	files, err := migrationFiles(driver)
	if err != nil {
		return err
	}

	for _, name := range files {
		version, err := parseVersion(name)
		if err != nil {
			return fmt.Errorf("parse version from %s: %w", name, err)
		}

		applied, err := isApplied(db, version)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if applied {
			continue
		}

		content, err := readMigration(driver, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.Exec(content); err != nil {
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		if err := markApplied(db, version); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func migrationFiles(driver string) ([]string, error) {
	var fs embed.FS
	switch driver {
	case "sqlite":
		fs = sqliteMigrations
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	entries, err := fs.ReadDir(driver + "/migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func parseVersion(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid migration name: %s", name)
	}
	return strconv.Atoi(parts[0])
}

func readMigration(driver, name string) (string, error) {
	var fs embed.FS
	switch driver {
	case "sqlite":
		fs = sqliteMigrations
	default:
		return "", fmt.Errorf("unsupported driver: %s", driver)
	}

	data, err := fs.ReadFile(driver + "/migrations/" + name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func isApplied(db *sql.DB, version int) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func markApplied(db *sql.DB, version int) error {
	_, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version)
	return err
}
