package db

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ipanalytics/rir-sql-forge/internal/parser"
)

func TestStoreFlattenAndCSV(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertNetworks(ctx, []parser.Network{
		{CIDR: "203.0.113.0/24", RIR: "RIPE", OrgID: "ORG-1", ContactID: "TECH-1"},
		{CIDR: "198.51.100.0/24", RIR: "RIPE", ContactID: "TECH-1"},
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertOrganisations(ctx, []parser.Organisation{{OrgID: "ORG-1", OrgName: "Example Ltd", Country: "DE", AbuseContactID: "ABUSE-1"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertContacts(ctx, []parser.Contact{
		{ContactID: "ABUSE-1", AbuseEmail: "abuse@example.net"},
		{ContactID: "TECH-1", TechEmail: "noc@example.net"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Flatten(ctx); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := store.WriteCSV(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	csv := buf.String()
	for _, want := range []string{"cidr,rir,org_name,country,contact_email", "abuse@example.net", "noc@example.net"} {
		if !strings.Contains(csv, want) {
			t.Fatalf("CSV missing %q:\n%s", want, csv)
		}
	}
}
