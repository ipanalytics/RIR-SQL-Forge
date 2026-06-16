package compiler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileFixtureResolvesOrgAndFallback(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.db")
	if err := os.WriteFile(fixture, []byte(`inetnum: 203.0.113.0 - 203.0.113.255
org: ORG-EXAMPLE-RIPE
admin-c: TECH1-RIPE

inetnum: 198.51.100.0 - 198.51.100.255
tech-c: TECH1-RIPE

inetnum: 192.0.2.0 - 192.0.2.255

organisation: ORG-EXAMPLE-RIPE
org-name: Example Networks Ltd
country: DE
abuse-c: ABUSE1-RIPE

role: Abuse Desk
nic-hdl: ABUSE1-RIPE
abuse-mailbox: abuse@example.net
e-mail: role@example.net

person: Tech Person
nic-hdl: TECH1-RIPE
e-mail: noc@example.net
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	err := Compile(context.Background(), Options{
		OutputDir:    filepath.Join(dir, "out"),
		WorkDir:      filepath.Join(dir, "work"),
		SkipDownload: true,
		LocalSources: []LocalSource{{RIR: "RIPE", Path: fixture}},
	}, &log)
	if err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(dir, "out", "net_owner_directory.csv")
	body, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	csv := string(body)
	for _, want := range []string{"abuse@example.net", "noc@example.net", "192.0.2.0/24,RIPE,,,"} {
		if !strings.Contains(csv, want) {
			t.Fatalf("CSV missing %q:\n%s", want, csv)
		}
	}
	if !strings.Contains(log.String(), "ARIN XML path not provided; skipping ARIN") || !strings.Contains(log.String(), "LACNIC bulk path not provided; skipping LACNIC") {
		t.Fatalf("manual source skip logs missing:\n%s", log.String())
	}
}

func TestCompileLACNICFixtureAndARINSkip(t *testing.T) {
	dir := t.TempDir()
	lacnic := filepath.Join(dir, "lacnic.db")
	if err := os.WriteFile(lacnic, []byte("route: 203.0.113.0/24\ntech-c: LACNIC1\n\nrole: LACNIC NOC\nnic-hdl: LACNIC1\ne-mail: noc@lacnic.example\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	err := Compile(context.Background(), Options{
		OutputDir:    filepath.Join(dir, "out"),
		WorkDir:      filepath.Join(dir, "work"),
		SkipDownload: true,
		LACNICDBPath: lacnic,
		ARINXMLPath:  filepath.Join(dir, "arin.xml"),
	}, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "ARIN XML parsing is not implemented") {
		t.Fatalf("ARIN skip log missing:\n%s", log.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "out", "net_owner_directory.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "noc@lacnic.example") {
		t.Fatalf("LACNIC CSV missing contact:\n%s", string(body))
	}
}
