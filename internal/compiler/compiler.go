package compiler

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipanalytics/rir-sql-forge/internal/db"
	"github.com/ipanalytics/rir-sql-forge/internal/downloader"
	"github.com/ipanalytics/rir-sql-forge/internal/parser"
	"github.com/ipanalytics/rir-sql-forge/internal/sources"
)

// LocalSource is a user-provided RPSL input file.
type LocalSource struct {
	RIR  string
	Path string
}

// Options configures a compile run.
type Options struct {
	OutputDir      string
	WorkDir        string
	SkipDownload   bool
	IncludeRIPE    bool
	IncludeAPNIC   bool
	IncludeAFRINIC bool
	ARINXMLPath    string
	LACNICDBPath   string
	LocalSources   []LocalSource
}

// Compile builds SQLite and CSV outputs from public or local RPSL sources.
func Compile(ctx context.Context, opts Options, log io.Writer) error {
	opts = opts.withDefaults()
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.WorkDir, 0o755); err != nil {
		return err
	}

	sourceList, err := prepareSources(ctx, opts, log)
	if err != nil {
		return err
	}
	sqlitePath := filepath.Join(opts.OutputDir, "net_owner_directory.sqlite")
	csvPath := filepath.Join(opts.OutputDir, "net_owner_directory.csv")
	_ = os.Remove(sqlitePath)

	store, err := db.Open(sqlitePath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		return err
	}

	if err := ingestSources(ctx, store, sourceList); err != nil {
		return err
	}
	if err := store.Flatten(ctx); err != nil {
		return err
	}
	if err := store.ExportCSV(ctx, csvPath); err != nil {
		return err
	}
	fmt.Fprintf(log, "wrote %s\n", sqlitePath)
	fmt.Fprintf(log, "wrote %s\n", csvPath)
	return nil
}

func (o Options) withDefaults() Options {
	if o.OutputDir == "" {
		o.OutputDir = "dist"
	}
	if o.WorkDir == "" {
		o.WorkDir = "tmp/rir-sql-forge"
	}
	return o
}

func prepareSources(ctx context.Context, opts Options, log io.Writer) ([]sources.Source, error) {
	var sourceList []sources.Source
	for _, local := range opts.LocalSources {
		if local.Path == "" {
			continue
		}
		rir := strings.ToUpper(strings.TrimSpace(local.RIR))
		if rir == "" {
			rir = "LOCAL"
		}
		sourceList = append(sourceList, sources.Source{RIR: rir, LocalPath: local.Path, Gzip: strings.HasSuffix(local.Path, ".gz"), UserProvided: true})
	}

	if opts.LACNICDBPath == "" {
		fmt.Fprintln(log, "LACNIC bulk path not provided; skipping LACNIC")
	} else {
		sourceList = append(sourceList, sources.Source{RIR: "LACNIC", LocalPath: opts.LACNICDBPath, Gzip: strings.HasSuffix(opts.LACNICDBPath, ".gz"), UserProvided: true})
	}
	if opts.ARINXMLPath == "" {
		fmt.Fprintln(log, "ARIN XML path not provided; skipping ARIN")
	} else {
		fmt.Fprintf(log, "ARIN XML parsing is not implemented for %s; skipping ARIN without failing public compilation\n", opts.ARINXMLPath)
	}

	if opts.SkipDownload {
		fmt.Fprintln(log, "public downloads skipped")
		return sourceList, nil
	}

	client := downloader.Client{}
	for _, src := range sources.Public(opts.IncludeRIPE, opts.IncludeAPNIC, opts.IncludeAFRINIC) {
		fmt.Fprintf(log, "downloading %s %s from %s\n", src.RIR, src.ObjectHint, src.URL)
		downloaded, err := client.Download(ctx, opts.WorkDir, src)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(log, "downloaded %s %s to %s\n", downloaded.RIR, downloaded.ObjectHint, downloaded.LocalPath)
		sourceList = append(sourceList, downloaded)
	}
	return sourceList, nil
}

func ingestSources(ctx context.Context, store *db.Store, sourceList []sources.Source) error {
	var networks []parser.Network
	orgs := map[string]parser.Organisation{}
	contacts := map[string]parser.Contact{}

	for _, src := range sourceList {
		records, err := parseSource(ctx, src)
		if err != nil {
			return fmt.Errorf("%s %s: %w", src.RIR, src.LocalPath, err)
		}
		for _, record := range records {
			networks = append(networks, record.Networks...)
			if record.Organisation != nil && record.Organisation.OrgID != "" {
				orgs[record.Organisation.OrgID] = *record.Organisation
			}
			if record.Contact != nil && record.Contact.ContactID != "" {
				contacts[record.Contact.ContactID] = *record.Contact
			}
		}
	}

	orgRows := make([]parser.Organisation, 0, len(orgs))
	for _, org := range orgs {
		orgRows = append(orgRows, org)
	}
	contactRows := make([]parser.Contact, 0, len(contacts))
	for _, contact := range contacts {
		contactRows = append(contactRows, contact)
	}
	if err := store.InsertNetworks(ctx, networks, 50000); err != nil {
		return err
	}
	if err := store.UpsertOrganisations(ctx, orgRows); err != nil {
		return err
	}
	return store.UpsertContacts(ctx, contactRows)
}

func parseSource(ctx context.Context, src sources.Source) ([]parser.Record, error) {
	file, err := os.Open(src.LocalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var reader io.Reader = file
	if src.Gzip || strings.HasSuffix(src.LocalPath, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	return parser.ParseAll(ctx, reader, src.RIR)
}
