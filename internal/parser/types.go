package parser

// Network is a normalized IP network row extracted from RPSL route or inet objects.
type Network struct {
	CIDR      string
	RIR       string
	OrgID     string
	ContactID string
}

// Organisation is an RPSL organisation object.
type Organisation struct {
	OrgID          string
	OrgName        string
	Country        string
	AbuseContactID string
}

// Contact is an RPSL role, person, or irt object.
type Contact struct {
	ContactID  string
	AbuseEmail string
	TechEmail  string
}

// Record is a typed projection of one parsed RPSL object.
type Record struct {
	ObjectType    string
	Networks      []Network
	Organisation  *Organisation
	Contact       *Contact
	RawAttributes map[string][]string
}
