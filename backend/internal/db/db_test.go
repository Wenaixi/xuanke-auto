package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAndSchema(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var n int
	d.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&n)
	if n < 4 {
		t.Fatalf("expected >=4 tables, got %d", n)
	}
}
