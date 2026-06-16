package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/ipanalytics/rir-sql-forge/internal/compiler"
)

const version = "dev"

// Main runs the command-line application and returns a process exit code.
func Main(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootHelp(stdout)
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		printRootHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "rir-sql-forge %s\n", version)
		return 0
	case "compile":
		return runCompile(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printRootHelp(stderr)
		return 2
	}
}

func printRootHelp(w io.Writer) {
	fmt.Fprintln(w, `RIR-SQL-Forge compiles RPSL bulk data into a local abuse-contact directory.

Usage:
  rir-sql-forge compile [flags]
  rir-sql-forge version

Commands:
  compile   Download or read RIR bulk data and build SQLite/CSV outputs
  version   Print the build version

Run "rir-sql-forge compile --help" for compile flags.`)
}

func runCompile(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var localInputs multiFlag
	opts := compiler.Options{}

	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.OutputDir, "output", "dist", "directory for generated SQLite and CSV assets")
	fs.StringVar(&opts.WorkDir, "work-dir", "tmp/rir-sql-forge", "directory for downloaded temporary source files")
	fs.BoolVar(&opts.SkipDownload, "skip-download", false, "skip public downloads and use --source fixtures or manual paths only")
	fs.BoolVar(&opts.IncludeRIPE, "ripe", true, "include RIPE public RPSL data")
	fs.BoolVar(&opts.IncludeAPNIC, "apnic", true, "include APNIC public RPSL data")
	fs.BoolVar(&opts.IncludeAFRINIC, "afrinic", true, "include AFRINIC public RPSL data")
	fs.StringVar(&opts.ARINXMLPath, "arin-xml", "", "optional local ARIN bulk XML path; skipped when empty")
	fs.StringVar(&opts.LACNICDBPath, "lacnic-db", "", "optional local LACNIC RPSL database path; skipped when empty")
	fs.Var(&localInputs, "source", "local RPSL source in RIR=PATH form; may be repeated and implies fixture/local ingestion")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	opts.LocalSources = parseLocalInputs(localInputs)
	if err := compiler.Compile(ctx, opts, stdout); err != nil {
		fmt.Fprintf(stderr, "compile failed: %v\n", err)
		return 1
	}
	return 0
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func parseLocalInputs(values []string) []compiler.LocalSource {
	out := make([]compiler.LocalSource, 0, len(values))
	for _, value := range values {
		rir, path, ok := strings.Cut(value, "=")
		if !ok {
			out = append(out, compiler.LocalSource{RIR: "LOCAL", Path: value})
			continue
		}
		out = append(out, compiler.LocalSource{RIR: strings.ToUpper(strings.TrimSpace(rir)), Path: strings.TrimSpace(path)})
	}
	return out
}
