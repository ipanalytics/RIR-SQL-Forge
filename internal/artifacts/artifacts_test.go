package artifacts

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipanalytics/rir-sql-forge/internal/db"
	_ "github.com/marcboeker/go-duckdb/v2"
)

func TestExportDuckDBAndParquet(t *testing.T) {
	rows := []db.DirectoryRow{
		{CIDR: "203.0.113.0/24", RIR: "RIPE", OrgName: "Example Ltd", Country: "DE", ContactEmail: "abuse@example.net"},
	}
	dir := t.TempDir()
	duckPath := filepath.Join(dir, "net_owner_directory.duckdb")
	parquetPath := filepath.Join(dir, "net_owner_directory.parquet")

	if err := ExportDuckDB(context.Background(), duckPath, rows); err != nil {
		t.Fatal(err)
	}
	if err := ExportParquet(parquetPath, rows); err != nil {
		t.Fatal(err)
	}

	if stat, err := os.Stat(parquetPath); err != nil || stat.Size() == 0 {
		t.Fatalf("parquet output missing or empty: stat=%v err=%v", stat, err)
	}

	conn, err := sql.Open("duckdb", duckPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var email string
	if err := conn.QueryRowContext(context.Background(), `SELECT contact_email FROM ip_owner_abuse_directory WHERE cidr = ?`, "203.0.113.0/24").Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email != "abuse@example.net" {
		t.Fatalf("email = %q", email)
	}
}
