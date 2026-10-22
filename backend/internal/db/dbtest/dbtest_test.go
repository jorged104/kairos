package dbtest

import (
	"context"
	"strings"
	"testing"
)

func TestNewCreatesMigratedDatabase(t *testing.T) {
	pool := New(t)
	if !strings.HasPrefix(pool.Config().ConnConfig.Database, "kairos_test_") {
		t.Fatalf("database = %q, want a kairos_test_ database", pool.Config().ConnConfig.Database)
	}
	var version int64
	if err := pool.QueryRow(context.Background(), "SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil {
		t.Fatalf("query goose version: %v", err)
	}
	if version < 1 {
		t.Fatalf("goose version = %d, want migrations applied", version)
	}
}
