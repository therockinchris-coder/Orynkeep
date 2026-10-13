package platform

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Requires a dedicated disposable database. This test deliberately changes
// migration history to verify that modified migrations are rejected.
func TestMigrationsIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migration run: %v", err)
	}
	for _, table := range []string{"users", "files", "documents", "photos"} {
		var found bool
		err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&found)
		if err != nil || !found {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	var original string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE name='001_initial.sql'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), `UPDATE schema_migrations SET checksum=$1 WHERE name='001_initial.sql'`, original); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.ExecContext(ctx, `UPDATE schema_migrations SET checksum='changed' WHERE name='001_initial.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("modified migration accepted")
	}
}
