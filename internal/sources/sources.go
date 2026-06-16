package sources

// Source describes one downloadable or local RIR data file.
type Source struct {
	RIR          string
	ObjectHint   string
	URL          string
	LocalPath    string
	Gzip         bool
	Combined     bool
	UserProvided bool
}

// Public returns the supported public RPSL bulk data sources.
func Public(includeRIPE, includeAPNIC, includeAFRINIC bool) []Source {
	var out []Source
	if includeRIPE {
		out = append(out,
			Source{RIR: "RIPE", ObjectHint: "inetnum", URL: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.inetnum.gz", Gzip: true},
			Source{RIR: "RIPE", ObjectHint: "inet6num", URL: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.inet6num.gz", Gzip: true},
			Source{RIR: "RIPE", ObjectHint: "organisation", URL: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.organisation.gz", Gzip: true},
			Source{RIR: "RIPE", ObjectHint: "role", URL: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.role.gz", Gzip: true},
			Source{RIR: "RIPE", ObjectHint: "person", URL: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.person.gz", Gzip: true},
		)
	}
	if includeAPNIC {
		out = append(out,
			Source{RIR: "APNIC", ObjectHint: "inetnum", URL: "https://ftp.apnic.net/apnic/whois/apnic.db.inetnum.gz", Gzip: true},
			Source{RIR: "APNIC", ObjectHint: "inet6num", URL: "https://ftp.apnic.net/apnic/whois/apnic.db.inet6num.gz", Gzip: true},
			Source{RIR: "APNIC", ObjectHint: "organisation", URL: "https://ftp.apnic.net/apnic/whois/apnic.db.organisation.gz", Gzip: true},
			Source{RIR: "APNIC", ObjectHint: "role", URL: "https://ftp.apnic.net/apnic/whois/apnic.db.role.gz", Gzip: true},
			Source{RIR: "APNIC", ObjectHint: "person", URL: "https://ftp.apnic.net/apnic/whois/apnic.db.person.gz", Gzip: true},
		)
	}
	if includeAFRINIC {
		out = append(out, Source{RIR: "AFRINIC", ObjectHint: "combined", URL: "https://ftp.afrinic.net/pub/dbase/afrinic.db.gz", Gzip: true, Combined: true})
	}
	return out
}
