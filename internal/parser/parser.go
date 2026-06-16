package parser

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

// Stream reads RPSL objects from r and sends typed records to out.
func Stream(ctx context.Context, r io.Reader, rir string, out chan<- Record) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	var paragraph []string
	flush := func() error {
		if len(paragraph) == 0 {
			return nil
		}
		record, ok, err := ParseParagraph(paragraph, rir)
		paragraph = nil
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		select {
		case out <- record:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		paragraph = append(paragraph, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

// StreamGzip reads gzip-compressed RPSL objects from r and sends typed records to out.
func StreamGzip(ctx context.Context, r io.Reader, rir string, out chan<- Record) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	return Stream(ctx, gz, rir, out)
}

// ParseParagraph parses one RPSL paragraph into a typed record.
func ParseParagraph(lines []string, rir string) (Record, bool, error) {
	attrs := map[string][]string{}
	var lastKey string
	for _, raw := range lines {
		if strings.HasPrefix(strings.TrimSpace(raw), "#") || strings.HasPrefix(strings.TrimSpace(raw), "%") {
			continue
		}
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			if lastKey != "" {
				values := attrs[lastKey]
				values[len(values)-1] = strings.TrimSpace(values[len(values)-1] + " " + strings.TrimSpace(raw))
				attrs[lastKey] = values
			}
			continue
		}
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		attrs[key] = append(attrs[key], value)
		lastKey = key
	}
	if len(attrs) == 0 {
		return Record{}, false, nil
	}

	objectType := detectObjectType(attrs)
	record := Record{ObjectType: objectType, RawAttributes: attrs}
	switch objectType {
	case "inetnum", "inet6num", "route", "route6":
		networks, err := parseNetworks(objectType, attrs, rir)
		if err != nil {
			return Record{}, false, err
		}
		record.Networks = networks
	case "organisation":
		record.Organisation = &Organisation{
			OrgID:          first(attrs, "organisation"),
			OrgName:        first(attrs, "org-name"),
			Country:        strings.ToUpper(first(attrs, "country")),
			AbuseContactID: first(attrs, "abuse-c"),
		}
	case "role", "person":
		record.Contact = &Contact{
			ContactID:  first(attrs, "nic-hdl"),
			AbuseEmail: first(attrs, "abuse-mailbox"),
			TechEmail:  first(attrs, "e-mail"),
		}
	default:
		return record, false, nil
	}
	return record, true, nil
}

func detectObjectType(attrs map[string][]string) string {
	for _, key := range []string{"inetnum", "inet6num", "route", "route6", "organisation", "role", "person"} {
		if _, ok := attrs[key]; ok {
			return key
		}
	}
	return ""
}

func parseNetworks(objectType string, attrs map[string][]string, rir string) ([]Network, error) {
	var cidrs []string
	var err error
	switch objectType {
	case "inetnum":
		cidrs, err = IPv4RangeToCIDRs(first(attrs, "inetnum"))
	case "inet6num":
		cidrs, err = normalizePrefixes(first(attrs, "inet6num"))
	case "route":
		cidrs, err = normalizePrefixes(first(attrs, "route"))
	case "route6":
		cidrs, err = normalizePrefixes(first(attrs, "route6"))
	}
	if err != nil {
		return nil, err
	}

	contactID := first(attrs, "admin-c")
	if contactID == "" {
		contactID = first(attrs, "tech-c")
	}
	networks := make([]Network, 0, len(cidrs))
	for _, cidr := range cidrs {
		networks = append(networks, Network{
			CIDR:      cidr,
			RIR:       rir,
			OrgID:     first(attrs, "org"),
			ContactID: contactID,
		})
	}
	return networks, nil
}

// IPv4RangeToCIDRs normalizes an RPSL inetnum range into one or more CIDR strings.
func IPv4RangeToCIDRs(value string) ([]string, error) {
	startText, endText, ok := strings.Cut(value, "-")
	if !ok {
		return normalizePrefixes(value)
	}
	start := net.ParseIP(strings.TrimSpace(startText)).To4()
	end := net.ParseIP(strings.TrimSpace(endText)).To4()
	if start == nil || end == nil {
		return nil, fmt.Errorf("invalid IPv4 range %q", value)
	}
	startN := uint64(ip4ToUint(start))
	endN := uint64(ip4ToUint(end))
	if startN > endN {
		return nil, fmt.Errorf("invalid descending IPv4 range %q", value)
	}
	var cidrs []string
	for startN <= endN {
		maxSize := startN & -startN
		if maxSize == 0 {
			maxSize = 1 << 32
		}
		remaining := endN - startN + 1
		for maxSize > remaining {
			maxSize >>= 1
		}
		prefixLen := 32 - bitsForBlock(maxSize)
		cidrs = append(cidrs, fmt.Sprintf("%s/%d", uintToIP4(uint32(startN)), prefixLen))
		startN += maxSize
	}
	return cidrs, nil
}

func normalizePrefixes(value string) ([]string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	return []string{prefix.Masked().String()}, nil
}

func ip4ToUint(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uintToIP4(value uint32) net.IP {
	return net.IPv4(byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
}

func bitsForBlock(block uint64) int {
	if block == 0 {
		return 32
	}
	bits := 0
	for block > 1 {
		block >>= 1
		bits++
	}
	return bits
}

func first(attrs map[string][]string, key string) string {
	values := attrs[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

// AttributeKeys returns sorted parsed attribute names for diagnostics and tests.
func AttributeKeys(record Record) []string {
	keys := make([]string, 0, len(record.RawAttributes))
	for key := range record.RawAttributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ParseAll is a test-friendly helper that collects a stream into memory.
func ParseAll(ctx context.Context, r io.Reader, rir string) ([]Record, error) {
	out := make(chan Record)
	var records []Record
	var streamErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamErr = Stream(ctx, r, rir, out)
		close(out)
	}()
	for record := range out {
		records = append(records, record)
	}
	<-done
	return records, streamErr
}

// ParseDecimal is kept small and explicit for tests that need stable numeric errors.
func ParseDecimal(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}
