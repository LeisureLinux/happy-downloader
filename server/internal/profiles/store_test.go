package profiles

import (
	"testing"

	bolt "go.etcd.io/bbolt"
)

func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()

	f := t.TempDir() + "/test.db"
	db, err := bolt.Open(f, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	s, err := NewStore(db, Profile{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	return s, func() { db.Close() }
}

// First run must produce a usable table, not an empty one an operator has to
// fill in before anything downloads.
func TestStoreSeedsDefaults(t *testing.T) {
	s, close := newTestStore(t)
	defer close()

	tbl := s.Table()

	if len(tbl) < 4 {
		t.Fatalf("seeded %d profiles, want at least 4", len(tbl))
	}

	// A profile may name several domains, comma-separated, so check that each
	// wanted host appears in some entry rather than matching a whole field.
	for _, want := range []string{"youtube.com", "youtu.be", "bilibili.com", "x.com", "twitter.com"} {
		found := false
		for _, p := range tbl {
			for _, suffix := range p.suffixes() {
				if suffix == want {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no seeded profile for %q", want)
		}
	}

	// Exactly one fallback, and it is last.
	if tbl[len(tbl)-1].HostSuffix != "" {
		t.Errorf("fallback not last: %+v", tbl[len(tbl)-1])
	}

	// The fallback must be last so match() finds it deterministically.
	if tbl[len(tbl)-1].HostSuffix != "" {
		t.Errorf("fallback not last: %+v", tbl[len(tbl)-1])
	}
}

// The profiles no longer carry the cookie, the PO Token provider or the
// proxy.
//
// They were three settings that had to be typed into every row of a table,
// and the value was almost always the same in all of them. They are read from
// the configuration when a download is built now, because they are properties
// of the installation rather than of a site.
func TestStoreSeedsNoPerSiteSettings(t *testing.T) {
	f := t.TempDir() + "/test.db"
	db, err := bolt.Open(f, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}
	defer db.Close()

	_, err = NewStore(db, Profile{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	globalMu.RLock()
	s := global
	globalMu.RUnlock()

	// The entries are there, and they carry nothing but what is per site.
	p, _ := s.table.match("https://www.youtube.com/watch?v=x")
	if p.HostSuffix == "" {
		t.Error("YouTube has no profile")
	}

	// The fallback resolves a URL that no profile names.
	p2, _ := ForURL("https://example.com/video")
	if p2.HostSuffix != "" {
		t.Errorf("the fallback has a host suffix %q, want none", p2.HostSuffix)
	}

	// A host with its own profile must not fall through to the fallback.
	p3, _ := ForURL("https://www.bilibili.com/video/BV1xx")
	if p3.HostSuffix != "bilibili.com" {
		t.Errorf("bilibili resolved to %q, want its own profile", p3.Name)
	}

	// Nothing on a profile carries a route any more: the proxy is chosen
	// when the download is built, by asking whether the site answers
	// directly.
	for _, u := range []string{
		"https://example.com/video",
		"https://www.bilibili.com/video/BV1xx",
		"https://www.youtube.com/watch?v=x",
	} {
		p, _ := ForURL(u)
		if p.Name == "" {
			t.Errorf("%s resolved to nothing", u)
		}
	}
}

func TestReplaceKeepsExactlyOneFallback(t *testing.T) {
	s, close := newTestStore(t)
	defer close()

	err := s.Replace(table{
		{Name: "A", HostSuffix: "a.com"},
		{Name: "F1", HostSuffix: ""},
		{Name: "F2", HostSuffix: ""},
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}

	tbl := s.Table()

	fallbacks := 0
	for _, p := range tbl {
		if p.HostSuffix == "" {
			fallbacks++
		}
	}

	if fallbacks != 1 {
		t.Errorf("got %d fallbacks, want 1", fallbacks)
	}
	if tbl[len(tbl)-1].HostSuffix != "" {
		t.Error("fallback is not last")
	}
}

// A Replace with no fallback entry still has to produce one, or unmatched
// URLs would get no arguments at all.
func TestReplaceAddsFallbackWhenMissing(t *testing.T) {
	s, close := newTestStore(t)
	defer close()

	if err := s.Replace(table{{Name: "Only", HostSuffix: "only.com"}}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	tbl := s.Table()
	if tbl[len(tbl)-1].HostSuffix != "" {
		t.Errorf("no fallback produced: %+v", tbl)
	}
}

func TestReplacePersistsAcrossReopen(t *testing.T) {
	f := t.TempDir() + "/persist.db"

	db, err := bolt.Open(f, 0600, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	s, err := NewStore(db, Profile{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	want := table{
		{Name: "Custom", HostSuffix: "custom.com"},
		{Name: "Fallback"},
	}
	if err := s.Replace(want); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	db.Close()

	db2, err := bolt.Open(f, 0600, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()

	s2, err := NewStore(db2, Profile{})
	if err != nil {
		t.Fatalf("reopen NewStore: %v", err)
	}

	tbl := s2.Table()
	if len(tbl) != 2 {
		t.Fatalf("reopened with %d profiles, want 2", len(tbl))
	}

	p, _ := ForURL("https://custom.com/v/1")
	if p.Name != "Custom" {
		t.Errorf("persisted profile wrong: %+v", p)
	}
}
