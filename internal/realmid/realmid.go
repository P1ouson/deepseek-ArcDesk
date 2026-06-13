package realmid

import (
	"fmt"
	"strconv"
	"strings"
)

// RealmID is the parsed form of a realm-prefixed node identifier.
// Wire format: "<realm>:<path>" or "<realm>:<path>#<symbol>" or
// "<realm>:<path>:<line>#<symbol>" for call sites.
type RealmID struct {
	Realm  string // go, js, ui, hook, gobind, ...
	Path   string // repo-relative path or module path
	Symbol string // optional, after #
	Line   int    // optional call-site line number before #
}

// Parse parses a NodeID string into RealmID.
func Parse(id string) (RealmID, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return RealmID{}, fmt.Errorf("empty node id")
	}
	realm, rest, ok := strings.Cut(id, ":")
	if !ok || realm == "" {
		return RealmID{}, fmt.Errorf("invalid node id %q: missing realm prefix", id)
	}
	if rest == "" {
		return RealmID{}, fmt.Errorf("invalid node id %q: empty path", id)
	}

	symbol := ""
	body := rest
	if before, after, ok := strings.Cut(rest, "#"); ok {
		body = before
		symbol = after
		if symbol == "" {
			return RealmID{}, fmt.Errorf("invalid node id %q: empty symbol", id)
		}
	}

	path, line := splitPathLine(body)
	if path == "" {
		return RealmID{}, fmt.Errorf("invalid node id %q: empty path", id)
	}

	return RealmID{
		Realm:  realm,
		Path:   path,
		Symbol: symbol,
		Line:   line,
	}, nil
}

// MustParse is like Parse but panics on error.
func MustParse(id string) RealmID {
	r, err := Parse(id)
	if err != nil {
		panic(err)
	}
	return r
}

// New creates a NodeID string from parts without a line number.
func New(realm, path, symbol string) string {
	return formatID(realm, path, 0, symbol)
}

// NewWithLine creates a NodeID string with an optional call-site line number.
func NewWithLine(realm, path string, line int, symbol string) string {
	return formatID(realm, path, line, symbol)
}

// String returns the wire-format NodeID.
func (r RealmID) String() string {
	return formatID(r.Realm, r.Path, r.Line, r.Symbol)
}

func formatID(realm, path string, line int, symbol string) string {
	realm = strings.TrimSpace(realm)
	path = strings.TrimSpace(path)
	symbol = strings.TrimSpace(symbol)

	var b strings.Builder
	b.WriteString(realm)
	b.WriteByte(':')
	b.WriteString(path)
	if line > 0 {
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(line))
	}
	if symbol != "" {
		b.WriteByte('#')
		b.WriteString(symbol)
	}
	return b.String()
}

func splitPathLine(body string) (path string, line int) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", 0
	}
	last := strings.LastIndexByte(body, ':')
	if last < 0 {
		return body, 0
	}
	tail := body[last+1:]
	n, err := strconv.Atoi(tail)
	if err != nil || n < 0 {
		return body, 0
	}
	return body[:last], n
}
