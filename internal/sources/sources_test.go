package sources

import "testing"

func TestPublicSources(t *testing.T) {
	got := Public(true, true, true)
	if len(got) != 11 {
		t.Fatalf("source count = %d", len(got))
	}
	want := []string{
		"https://ftp.ripe.net/ripe/dbase/split/ripe.db.inetnum.gz",
		"https://ftp.ripe.net/ripe/dbase/split/ripe.db.person.gz",
		"https://ftp.apnic.net/apnic/whois/apnic.db.organisation.gz",
		"https://ftp.apnic.net/apnic/whois/apnic.db.irt.gz",
		"https://ftp.afrinic.net/pub/dbase/afrinic.db.gz",
	}
	for _, url := range want {
		found := false
		for _, src := range got {
			found = found || src.URL == url
		}
		if !found {
			t.Fatalf("missing %s in %#v", url, got)
		}
	}
	if !got[len(got)-1].Combined {
		t.Fatal("AFRINIC source should be marked combined")
	}
}
