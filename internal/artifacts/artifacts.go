package artifacts

import (
	"context"
	"database/sql"
	"os"

	"github.com/ipanalytics/rir-sql-forge/internal/db"
	_ "github.com/marcboeker/go-duckdb/v2"
	"github.com/parquet-go/parquet-go"
)

// ExportParquet writes rows to a Parquet file.
func ExportParquet(path string, rows []db.DirectoryRow) error {
	_ = os.Remove(path)
	return parquet.WriteFile(path, rows)
}

// ExportDuckDB writes rows to a persistent DuckDB database file.
func ExportDuckDB(ctx context.Context, path string, rows []db.DirectoryRow) error {
	_ = os.Remove(path)
	conn, err := sql.Open("duckdb", path)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE ip_owner_abuse_directory (
		cidr VARCHAR,
		rir VARCHAR,
		org_name VARCHAR,
		country VARCHAR,
		contact_email VARCHAR
	)`); err != nil {
		return err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO ip_owner_abuse_directory(cidr, rir, org_name, country, contact_email) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err := stmt.ExecContext(ctx, row.CIDR, row.RIR, row.OrgName, row.Country, row.ContactEmail); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
