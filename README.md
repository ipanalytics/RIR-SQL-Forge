# RIR-SQL-Forge

RIR-SQL-Forge is a Go utility for building a local IP ownership and abuse-contact directory from public RPSL bulk data. It downloads RIPE, APNIC, and AFRINIC databases automatically, parses the data as streams, resolves organisation/contact relationships, and publishes SQLite, DuckDB, Parquet, and CSV artifacts suitable for security operations, abuse handling, risk systems, and network analytics.



<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License"></a>
  <img src="https://img.shields.io/badge/status-active-success" alt="Project status">
  <img src="https://img.shields.io/badge/go-1.26+-00ADD8" alt="Go version">
  <img src="https://img.shields.io/badge/database-SQLite%20%7C%20DuckDB-003B57" alt="Databases">
  <img src="https://img.shields.io/badge/format-Parquet-4c1" alt="Parquet">
  <img src="https://img.shields.io/badge/dataset-RIPE%20%7C%20APNIC%20%7C%20AFRINIC-4c1" alt="Datasets">
  <img src="https://img.shields.io/badge/CI-GitHub%20Actions-2088FF" alt="CI">
  <img src="https://img.shields.io/badge/releases-db--YYYY--MM--DD-informational" alt="Release format">
</p>

> Documentation: this README. Automated datasets: GitHub Releases produced by `.github/workflows/build-db.yml`.

---

## Overview

Regional Internet Registry bulk data is operationally useful, but the raw format is not query-friendly. Network objects do not usually contain direct abuse addresses; contacts are normalized into separate RPSL objects and linked by handles. RIR-SQL-Forge turns those raw registries into a local database that can be queried without live WHOIS lookups.

The compiler currently targets public RPSL data from:

| Registry | Retrieval | Inputs |
| --- | --- | --- |
| RIPE NCC | automatic download | `inetnum`, `inet6num`, `organisation`, `role` split files |
| APNIC | automatic download | `inetnum`, `inet6num`, `organisation`, `role` split files |
| AFRINIC | automatic download | combined `afrinic.db.gz` |
| LACNIC | local file | `--lacnic-db /path/to/lacnic.db` |
| ARIN | local file flag | `--arin-xml /path/to/arin.xml`; XML parsing is intentionally not automated in this MVP |

By default, `rir-sql-forge compile` downloads public RIPE, APNIC, and AFRINIC data itself. Local/manual sources are additive and are skipped when not provided.

---

## Architecture

```text
public RIR FTP/HTTPS
        |
        v
 streaming downloader  ---> temp source files
        |
        v
 RPSL stream parser     ---> networks / organisations / contacts
        |
        v
 SQLite bulk writer     ---> normalized tables
        |
        v
 flatten SQL            ---> SQLite / DuckDB / Parquet / CSV artifacts
```

RPSL relationship resolution follows registry references rather than text scraping:

```text
network.org
  -> organisation.organisation
  -> organisation.abuse-c
  -> role/person.nic-hdl
  -> abuse-mailbox
```

When `org` is absent, the compiler falls back to the direct operational contacts on the network object:

```text
network.admin-c or network.tech-c
  -> role/person.nic-hdl
  -> e-mail
```

The parser reads RPSL paragraphs from `io.Reader`, supports gzip input, handles continuation lines, and emits typed records into the compiler pipeline. SQLite writes use prepared statements and transactions with a 50,000-row default batch size.

---

## Features

- Automatic public bulk downloads for RIPE, APNIC, and AFRINIC.
- Streaming RPSL parser for plain and gzip input.
- Normalization for `inetnum`, `inet6num`, `route`, and `route6`.
- Relational contact resolution across network, organisation, role, and person objects.
- SQLite output using `modernc.org/sqlite` as the compatibility baseline.
- DuckDB output for local analytics and scan-heavy workflows.
- Parquet output for lakehouse, object storage, and columnar ingestion pipelines.
- Flat CSV export for downstream systems that do not need typed columnar data.
- Optional local LACNIC RPSL input.
- Explicit ARIN XML flag with safe skip behavior.
- Weekly GitHub Actions build that publishes compressed database artifacts.
- Fixture-backed tests for parser behavior, downloader failure paths, join logic, CSV export, and workflow syntax.

---

## Quick Start

```bash
git clone https://github.com/ipanalytics/rir-sql-forge.git
cd rir-sql-forge

go test ./...
go build ./cmd/rir-sql-forge

./rir-sql-forge compile --output dist --work-dir tmp/rir-sql-forge
```

The compile command downloads the public RIPE, APNIC, and AFRINIC datasets automatically unless `--skip-download` is set.

Generated files:

```text
dist/net_owner_directory.sqlite
dist/net_owner_directory.duckdb
dist/net_owner_directory.parquet
dist/net_owner_directory.csv
dist/net_owner_directory.stats.json
dist/net_owner_directory.stats.md
```

---

## Installation

### From source

```bash
go install github.com/ipanalytics/rir-sql-forge/cmd/rir-sql-forge@latest
```

### Local build

```bash
go build -o rir-sql-forge ./cmd/rir-sql-forge
```

### Development checks

```bash
go test ./...
go test -race ./...
go vet ./...
```

---

## Usage

### Build from public registries

```bash
rir-sql-forge compile \
  --output dist \
  --work-dir tmp/rir-sql-forge
```

This is the normal production path. The compiler downloads public RIPE, APNIC, and AFRINIC source files, parses them, and writes SQLite, DuckDB, Parquet, and CSV outputs.

### Use a local fixture or mirror

```bash
rir-sql-forge compile \
  --skip-download \
  --source RIPE=internal/compiler/testdata/sample.db \
  --output dist
```

### Include manual LACNIC data

```bash
rir-sql-forge compile \
  --lacnic-db /data/rir/lacnic.db \
  --output dist
```

### Provide an ARIN XML path

```bash
rir-sql-forge compile \
  --arin-xml /data/rir/arin.xml \
  --output dist
```

The current build logs a clear ARIN skip message rather than attempting credentialed ARIN automation.

### Disable a public registry

```bash
rir-sql-forge compile --afrinic=false
```

---

## Outputs and Artifacts

| Artifact | Produced by | Description |
| --- | --- | --- |
| `net_owner_directory.sqlite` | local compile / CI | Normalized SQLite database plus flat lookup table |
| `net_owner_directory.duckdb` | local compile / CI | DuckDB database containing `ip_owner_abuse_directory` |
| `net_owner_directory.parquet` | local compile / CI | Columnar Parquet export of the flat directory |
| `net_owner_directory.csv` | local compile / CI | CSV export of `ip_owner_abuse_directory` |
| `net_owner_directory.stats.json` | local compile / CI | Machine-readable dataset saturation report |
| `net_owner_directory.stats.md` | local compile / CI | Markdown coverage summary used in release notes |
| `net_owner_directory.sqlite.gz` | GitHub Actions | Compressed release database |
| `net_owner_directory.duckdb.gz` | GitHub Actions | Compressed DuckDB release database |
| `net_owner_directory.parquet` | GitHub Actions | Parquet release artifact |
| `net_owner_directory.csv.gz` | GitHub Actions | Compressed release CSV |

GitHub Releases use tags in the form:

```text
db-YYYY-MM-DD
```

---

## Data Format

The SQLite database contains normalized source tables and the flat table. DuckDB and Parquet contain the flat operational dataset.

SQLite schema:

```sql
CREATE TABLE networks (
    cidr TEXT,
    rir TEXT,
    org_id TEXT,
    contact_id TEXT
);

CREATE TABLE organisations (
    org_id TEXT PRIMARY KEY,
    org_name TEXT,
    country TEXT,
    abuse_contact_id TEXT
);

CREATE TABLE contacts (
    contact_id TEXT PRIMARY KEY,
    abuse_email TEXT,
    tech_email TEXT
);
```

The compiler then builds the flat operational table:

```sql
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
```

CSV columns:

| Column | Meaning |
| --- | --- |
| `cidr` | Normalized network prefix |
| `rir` | Source registry label |
| `org_name` | Organisation name when resolved |
| `country` | Organisation country code when present |
| `contact_email` | Abuse mailbox preferred; fallback technical email otherwise |

<details>
<summary>Source object fields used by the compiler</summary>

| Object | Fields |
| --- | --- |
| `inetnum`, `inet6num`, `route`, `route6` | `org`, `admin-c`, `tech-c` |
| `organisation` | `organisation`, `org-name`, `country`, `abuse-c` |
| `role`, `person` | `nic-hdl`, `abuse-mailbox`, `e-mail` |

</details>

---

## Dataset Saturation

Every compile writes a saturation report next to the data artifacts:

```text
net_owner_directory.stats.json
net_owner_directory.stats.md
```

The report is also embedded into GitHub Release notes. It is meant to answer the operational question that matters for this dataset: how much of the registry space resolved to useful ownership/contact data.

Tracked metrics:

| Metric | Meaning |
| --- | --- |
| `total_networks` | Rows in the flattened directory |
| `networks_with_email` | Rows where `contact_email` is populated |
| `email_coverage_pct` | Contact-email saturation across all network rows |
| `networks_with_org_name` | Rows resolved to an organisation name |
| `org_name_coverage_pct` | Organisation-name saturation |
| `abuse_mailbox_matches` | Rows resolved through `organisation.abuse-c -> abuse-mailbox` |
| `fallback_tech_email_matches` | Rows resolved through `admin-c` / `tech-c -> e-mail` |
| `contact_rows` | Parsed `role` and `person` contact rows |

Low coverage is useful signal, not hidden failure. It usually means the upstream registry objects do not expose contacts, the reference points to ARIN/LACNIC material that was not provided, or the registry data is incomplete for that range.

---

## Operational Notes

- Public registry downloads are network-bound and should run in CI or on a host with stable outbound access.
- Use `--work-dir` on persistent runners to control temporary source storage.
- The compiler removes and recreates the SQLite output file on each run.
- Tests do not require live registry access; they use local fixtures and fake HTTP transports.
- Release automation uses `GITHUB_TOKEN` and repository `contents: write` permission.
- For reproducible internal pipelines, mirror source files and run with `--skip-download --source RIR=/path/to/file`.

---

## Project Scope

RIR-SQL-Forge focuses on turning bulk registry data into local query artifacts. It is designed for scheduled offline builds, not live WHOIS serving.

In scope:

- public RIPE/APNIC/AFRINIC RPSL ingestion
- local/manual source files
- SQLite, DuckDB, Parquet, and CSV outputs
- CI-driven release asset publishing
- parser and join correctness tests

Out of scope for the current build:

- credential management for restricted registry portals
- hosted lookup APIs
- real-time WHOIS querying
- full ARIN XML normalization

---

## Use Cases

- abuse desk enrichment and routing
- fraud and risk scoring pipelines
- security telemetry enrichment
- network ownership analytics
- offline investigations where live WHOIS calls are slow or rate-limited
- scheduled internal datasets for SOC, CTI, and infrastructure teams

---

## Limitations

- Registry data quality varies by source and by object owner.
- Some networks do not resolve to an email address after RPSL joins.
- ARIN bulk XML support is represented by a local flag but not parsed in this version.
- DuckDB and Parquet contain the flat operational view; use SQLite when preserving normalized source joins matters.

---

## Directory Structure

```text
.
├── cmd/rir-sql-forge/          # CLI entrypoint
├── internal/app/               # Command parsing and application wiring
├── internal/compiler/          # End-to-end compile orchestration
├── internal/artifacts/         # DuckDB and Parquet artifact writers
├── internal/db/                # SQLite schema, bulk inserts, flattening, CSV export
├── internal/downloader/        # Streaming HTTP downloads
├── internal/parser/            # RPSL stream parser and CIDR normalization
├── internal/sources/           # Public RIR source catalog
├── .github/workflows/          # Weekly release automation
└── site/                       # Repository visual assets
```

---

## Deployment

The repository includes `.github/workflows/build-db.yml`.

Schedule:

```yaml
cron: '0 0 * * 0'
```

The workflow:

1. checks out the repository
2. installs Go
3. runs tests
4. builds `rir-sql-forge`
5. downloads and compiles public RIPE/APNIC/AFRINIC data
6. writes SQLite, DuckDB, Parquet, CSV, and saturation reports
7. compresses SQLite, DuckDB, and CSV outputs
8. embeds saturation stats into release notes
9. creates or updates a GitHub Release tagged `db-YYYY-MM-DD`

Manual runs are available through `workflow_dispatch`.

---

## License

RIR-SQL-Forge is licensed under the [Apache License 2.0](./LICENSE).

---

## Disclaimer

RIR-SQL-Forge processes registry data as published by the relevant RIRs. Review each registry's terms before redistributing derived datasets outside your organisation.
