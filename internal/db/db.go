package db

import (
	"context"
	"database/sql"
	"encoding/csv"
	"io"
	"os"

	"github.com/ipanalytics/rir-sql-forge/internal/parser"
	_ "modernc.org/sqlite"
)

const defaultBatchSize = 50000

// Store wraps a SQLite database used by the compiler.
type Store struct {
	db *sql.DB
}

// Open opens or creates a SQLite store at path.
func Open(path string) (*Store, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &Store{db: conn}, nil
}

// Close closes the underlying SQLite connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Init creates the database schema.
func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	return err
}

// InsertNetworks inserts network rows with batched transactions.
func (s *Store) InsertNetworks(ctx context.Context, rows []parser.Network, batchSize int) error {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	for start := 0; start < len(rows); start += batchSize {
		end := min(start+batchSize, len(rows))
		if err := s.withTx(ctx, func(tx *sql.Tx) error {
			stmt, err := tx.PrepareContext(ctx, `INSERT INTO networks(cidr, rir, org_id, contact_id) VALUES (?, ?, ?, ?)`)
			if err != nil {
				return err
			}
			defer stmt.Close()
			for _, row := range rows[start:end] {
				if _, err := stmt.ExecContext(ctx, row.CIDR, row.RIR, row.OrgID, row.ContactID); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// UpsertOrganisations stores organisations by primary key.
func (s *Store) UpsertOrganisations(ctx context.Context, rows []parser.Organisation) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO organisations(org_id, org_name, country, abuse_contact_id) VALUES (?, ?, ?, ?)
ON CONFLICT(org_id) DO UPDATE SET org_name=excluded.org_name, country=excluded.country, abuse_contact_id=excluded.abuse_contact_id`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, row := range rows {
			if row.OrgID == "" {
				continue
			}
			if _, err := stmt.ExecContext(ctx, row.OrgID, row.OrgName, row.Country, row.AbuseContactID); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpsertContacts stores contacts by primary key.
func (s *Store) UpsertContacts(ctx context.Context, rows []parser.Contact) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO contacts(contact_id, abuse_email, tech_email) VALUES (?, ?, ?)
ON CONFLICT(contact_id) DO UPDATE SET abuse_email=excluded.abuse_email, tech_email=excluded.tech_email`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, row := range rows {
			if row.ContactID == "" {
				continue
			}
			if _, err := stmt.ExecContext(ctx, row.ContactID, row.AbuseEmail, row.TechEmail); err != nil {
				return err
			}
		}
		return nil
	})
}

// Flatten rebuilds the flat ip_owner_abuse_directory table.
func (s *Store) Flatten(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, flattenSQL)
	return err
}

// ExportCSV writes the flattened table to path.
func (s *Store) ExportCSV(ctx context.Context, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return s.WriteCSV(ctx, file)
}

// WriteCSV writes the flattened table as CSV to w.
func (s *Store) WriteCSV(ctx context.Context, w io.Writer) error {
	rows, err := s.db.QueryContext(ctx, `SELECT cidr, rir, org_name, country, contact_email FROM ip_owner_abuse_directory ORDER BY cidr, rir`)
	if err != nil {
		return err
	}
	defer rows.Close()
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"cidr", "rir", "org_name", "country", "contact_email"}); err != nil {
		return err
	}
	for rows.Next() {
		var cidr, rir, orgName, country, email sql.NullString
		if err := rows.Scan(&cidr, &rir, &orgName, &country, &email); err != nil {
			return err
		}
		if err := cw.Write([]string{cidr.String, rir.String, orgName.String, country.String, email.String}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

const schemaSQL = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS networks (
    cidr TEXT,
    rir TEXT,
    org_id TEXT,
    contact_id TEXT
);
CREATE TABLE IF NOT EXISTS organisations (
    org_id TEXT PRIMARY KEY,
    org_name TEXT,
    country TEXT,
    abuse_contact_id TEXT
);
CREATE TABLE IF NOT EXISTS contacts (
    contact_id TEXT PRIMARY KEY,
    abuse_email TEXT,
    tech_email TEXT
);

CREATE INDEX IF NOT EXISTS idx_networks_org_id ON networks(org_id);
CREATE INDEX IF NOT EXISTS idx_networks_contact_id ON networks(contact_id);
CREATE INDEX IF NOT EXISTS idx_orgs_abuse_contact_id ON organisations(abuse_contact_id);
`

const flattenSQL = `
DROP TABLE IF EXISTS ip_owner_abuse_directory;
CREATE TABLE ip_owner_abuse_directory AS
SELECT
    n.cidr,
    n.rir,
    o.org_name,
    o.country,
    COALESCE(c1.abuse_email, c2.tech_email) AS contact_email
FROM networks n
LEFT JOIN organisations o ON n.org_id = o.org_id
LEFT JOIN contacts c1 ON o.abuse_contact_id = c1.contact_id
LEFT JOIN contacts c2 ON n.contact_id = c2.contact_id;
`
