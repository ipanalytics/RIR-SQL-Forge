package parser

import (
	"bytes"
	"compress/gzip"
	"context"
	"strings"
	"testing"
)

func TestParseRPSLObjects(t *testing.T) {
	input := `inetnum: 203.0.113.0 - 203.0.113.255
org: ORG-EXAMPLE-RIPE
admin-c: ADM1-RIPE

organisation: ORG-EXAMPLE-RIPE
org-name: Example
  Networks Ltd
country: de
abuse-c: AB1-RIPE

role: Abuse Role
nic-hdl: AB1-RIPE
abuse-mailbox: abuse@example.net
e-mail: noc@example.net
`
	records, err := ParseAll(context.Background(), strings.NewReader(input), "RIPE")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %d", len(records))
	}
	if got := records[0].Networks[0].CIDR; got != "203.0.113.0/24" {
		t.Fatalf("cidr = %s", got)
	}
	if got := records[1].Organisation.OrgName; got != "Example Networks Ltd" {
		t.Fatalf("org name = %q", got)
	}
	if got := records[2].Contact.AbuseEmail; got != "abuse@example.net" {
		t.Fatalf("abuse email = %q", got)
	}
}

func TestParseAPNICIRTContact(t *testing.T) {
	input := `inetnum: 203.0.113.0 - 203.0.113.255
mnt-irt: IRT-EXAMPLE-AP

irt: IRT-EXAMPLE-AP
abuse-mailbox: abuse-apnic@example.net
e-mail: noc-apnic@example.net
`
	records, err := ParseAll(context.Background(), strings.NewReader(input), "APNIC")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d", len(records))
	}
	if got := records[0].Networks[0].ContactID; got != "IRT-EXAMPLE-AP" {
		t.Fatalf("network contact = %q", got)
	}
	if got := records[1].Contact.AbuseEmail; got != "abuse-apnic@example.net" {
		t.Fatalf("irt abuse email = %q", got)
	}
}

func TestParseNetworkDirectAbuseC(t *testing.T) {
	input := `inetnum: 203.0.113.0 - 203.0.113.255
abuse-c: ABUSE-DIRECT
mnt-irt: IRT-SHOULD-NOT-WIN
tech-c: TECH-SHOULD-NOT-WIN
`
	records, err := ParseAll(context.Background(), strings.NewReader(input), "APNIC")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d", len(records))
	}
	if got := records[0].Networks[0].ContactID; got != "ABUSE-DIRECT" {
		t.Fatalf("network contact = %q", got)
	}
}

func TestGzipStream(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("route6: 2001:db8::/32\ntech-c: T1-TEST\n\n")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	out := make(chan Record)
	var records []Record
	errc := make(chan error, 1)
	go func() {
		errc <- StreamGzip(context.Background(), &buf, "TEST", out)
		close(out)
	}()
	for record := range out {
		records = append(records, record)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Networks[0].CIDR != "2001:db8::/32" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestIPv4RangeToCIDRs(t *testing.T) {
	tests := map[string][]string{
		"192.0.2.0 - 192.0.2.255": {"192.0.2.0/24"},
		"192.0.2.0 - 192.0.2.1":   {"192.0.2.0/31"},
		"192.0.2.1 - 192.0.2.1":   {"192.0.2.1/32"},
	}
	for input, want := range tests {
		got, err := IPv4RangeToCIDRs(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: got %v want %v", input, got, want)
		}
	}
	if _, err := IPv4RangeToCIDRs("192.0.2.10 - 192.0.2.1"); err == nil {
		t.Fatal("expected descending range error")
	}
}
