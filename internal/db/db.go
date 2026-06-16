package db

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
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

// DirectoryRow is one flattened ownership/contact row.
type DirectoryRow struct {
	CIDR         string `parquet:"cidr,zstd"`
	RIR          string `parquet:"rir,zstd"`
	OrgName      string `parquet:"org_name,zstd"`
	Country      string `parquet:"country,zstd"`
	ContactEmail string `parquet:"contact_email,zstd"`
}

// Stats describes dataset saturation and join quality.
type Stats struct {
	TotalNetworks         int64   `json:"total_networks"`
	NetworksWithOrgName   int64   `json:"networks_with_org_name"`
	NetworksWithCountry   int64   `json:"networks_with_country"`
	NetworksWithEmail     int64   `json:"networks_with_email"`
	NetworksWithoutEmail  int64   `json:"networks_without_email"`
	OrgNameCoveragePct    float64 `json:"org_name_coverage_pct"`
	CountryCoveragePct    float64 `json:"country_coverage_pct"`
	EmailCoveragePct      float64 `json:"email_coverage_pct"`
	AbuseMailboxMatches   int64   `json:"abuse_mailbox_matches"`
	FallbackTechMatches   int64   `json:"fallback_tech_email_matches"`
	NormalisedNetworkRows int64   `json:"normalised_network_rows"`
	OrganisationRows      int64   `json:"organisation_rows"`
	ContactRows           int64   `json:"contact_rows"`
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

// ExportStats writes machine-readable and Markdown dataset saturation reports.
func (s *Store) ExportStats(ctx context.Context, jsonPath, markdownPath string) (Stats, error) {
	stats, err := s.Stats(ctx)
	if err != nil {
		return Stats{}, err
	}
	body, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return Stats{}, err
	}
	body = append(body, '\n')
	if err := os.WriteFile(jsonPath, body, 0o644); err != nil {
		return Stats{}, err
	}
	if err := os.WriteFile(markdownPath, []byte(stats.Markdown()), 0o644); err != nil {
		return Stats{}, err
	}
	return stats, nil
}

// Stats calculates dataset saturation from normalized and flattened tables.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM networks`).Scan(&stats.NormalisedNetworkRows); err != nil {
		return Stats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organisations`).Scan(&stats.OrganisationRows); err != nil {
		return Stats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contacts`).Scan(&stats.ContactRows); err != nil {
		return Stats{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN org_name IS NOT NULL AND org_name <> '' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN country IS NOT NULL AND country <> '' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN contact_email IS NOT NULL AND contact_email <> '' THEN 1 ELSE 0 END), 0)
	FROM ip_owner_abuse_directory`)
	if err := row.Scan(&stats.TotalNetworks, &stats.NetworksWithOrgName, &stats.NetworksWithCountry, &stats.NetworksWithEmail); err != nil {
		return Stats{}, err
	}
	stats.NetworksWithoutEmail = stats.TotalNetworks - stats.NetworksWithEmail
	stats.OrgNameCoveragePct = percentage(stats.NetworksWithOrgName, stats.TotalNetworks)
	stats.CountryCoveragePct = percentage(stats.NetworksWithCountry, stats.TotalNetworks)
	stats.EmailCoveragePct = percentage(stats.NetworksWithEmail, stats.TotalNetworks)

	matchRow := s.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN c1.abuse_email IS NOT NULL AND c1.abuse_email <> '' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN (c1.abuse_email IS NULL OR c1.abuse_email = '') AND c2.tech_email IS NOT NULL AND c2.tech_email <> '' THEN 1 ELSE 0 END), 0)
	FROM networks n
	LEFT JOIN organisations o ON n.org_id = o.org_id
	LEFT JOIN contacts c1 ON o.abuse_contact_id = c1.contact_id
	LEFT JOIN contacts c2 ON n.contact_id = c2.contact_id`)
	if err := matchRow.Scan(&stats.AbuseMailboxMatches, &stats.FallbackTechMatches); err != nil {
		return Stats{}, err
	}
	return stats, nil
}

// Markdown renders a concise release-ready saturation report.
func (s Stats) Markdown() string {
	return fmt.Sprintf(`# Dataset saturation

| Metric | Value |
| --- | ---: |
| Total network rows | %d |
| Rows with contact email | %d |
| Rows without contact email | %d |
| Email coverage | %.2f%% |
| Rows with organisation name | %d |
| Organisation coverage | %.2f%% |
| Rows with country | %d |
| Country coverage | %.2f%% |
| Abuse mailbox matches | %d |
| Fallback tech/admin email matches | %d |
| Normalized network rows | %d |
| Organisation rows | %d |
| Contact rows | %d |

Low contact coverage usually means the upstream RPSL objects do not expose an abuse mailbox or the contact reference points to a restricted/manual registry source.
`,
		s.TotalNetworks,
		s.NetworksWithEmail,
		s.NetworksWithoutEmail,
		s.EmailCoveragePct,
		s.NetworksWithOrgName,
		s.OrgNameCoveragePct,
		s.NetworksWithCountry,
		s.CountryCoveragePct,
		s.AbuseMailboxMatches,
		s.FallbackTechMatches,
		s.NormalisedNetworkRows,
		s.OrganisationRows,
		s.ContactRows,
	)
}

// FlatRows returns the flattened directory rows in deterministic order.
func (s *Store) FlatRows(ctx context.Context) ([]DirectoryRow, error) {
	rows, err := s.queryFlatRows(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DirectoryRow
	for rows.Next() {
		row, err := scanDirectoryRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// WriteCSV writes the flattened table as CSV to w.
func (s *Store) WriteCSV(ctx context.Context, w io.Writer) error {
	rows, err := s.queryFlatRows(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"cidr", "rir", "org_name", "country", "contact_email"}); err != nil {
		return err
	}
	for rows.Next() {
		row, err := scanDirectoryRow(rows)
		if err != nil {
			return err
		}
		if err := cw.Write([]string{row.CIDR, row.RIR, row.OrgName, row.Country, row.ContactEmail}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

func (s *Store) queryFlatRows(ctx context.Context) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, `SELECT cidr, rir, org_name, country, contact_email FROM ip_owner_abuse_directory ORDER BY cidr, rir`)
}

func scanDirectoryRow(rows *sql.Rows) (DirectoryRow, error) {
	var cidr, rir, orgName, country, email sql.NullString
	if err := rows.Scan(&cidr, &rir, &orgName, &country, &email); err != nil {
		return DirectoryRow{}, err
	}
	return DirectoryRow{
		CIDR:         cidr.String,
		RIR:          rir.String,
		OrgName:      orgName.String,
		Country:      country.String,
		ContactEmail: email.String,
	}, nil
}

func percentage(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
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
