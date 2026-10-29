package cookies

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// What a cookie file holds, per site.
//
// The file is one list of every site's cookies, written by a browser export,
// and the question a person actually has is per-site: which site's session is
// about to be replaced, and which has not been refreshed at all. A single
// figure for the whole file cannot answer that — a file can be enormous and
// have nothing for the site that is failing.
//
// Only the shape is reported. A cookie value is a live credential, and none of
// this leaves the process.

// DomainSummary is what the file holds for one domain.
type DomainSummary struct {
	// Domain is the cookie's domain as the file spells it, without the
	// leading dot.
	Domain string
	// Count is how many cookies there are for it.
	Count int
	// Session is how many of them have no expiry.
	Session int
	// Expired is how many have already passed their expiry.
	Expired int
	// NewestExpiry is the latest expiry among them. Zero when every cookie
	// for the domain is a session cookie.
	NewestExpiry time.Time
	// MissingLogin lists the login cookies this domain's rule requires and
	// the file does not have. Empty for a site with no rule, or one that
	// satisfies it.
	//
	// It is the import rule, which is strict on purpose: it refuses a file
	// that would be a guest session. KeyCookie is the different, softer
	// question of what a person wants to see.
	MissingLogin []string

	// KeyCookie is the session cookie for this domain's site, when the site
	// is one whose name is known.
	//
	// Present says whether the file has it at all: absent means the export
	// is signed out for that site, which is the thing that makes a download
	// fail. Expiry is when it dies, zero for a session cookie.
	KeyCookie KeyCookie
}

// KeyCookie is the state of one site's session cookie.
type KeyCookie struct {
	// Name is the cookie the site uses for its session, e.g. "SESSDATA".
	Name string
	// Known is false when the site is not one this package knows a name
	// for, in which case the rest of the fields mean nothing.
	Known bool
	// Present is whether the file has the cookie at all.
	Present bool
	// Expiry is when it dies. Zero for a session cookie, which has no date.
	Expiry time.Time
	// Expired is whether that date has passed.
	Expired bool
}

// saneExpiry is the furthest future an expiry is believed.
//
// A cookie set to outlive the year 2100 was not set by a browser: the value
// is in the wrong unit, or the export wrote a sentinel. Reporting it as a date
// gives the interface a year with nine digits in it.
var saneExpiry = time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC)

// SummarizeDomains groups a Netscape file by domain.
//
// The order is by how many cookies each domain has, most first, so the sites
// that are actually in use come before the ones a browser merely visited.
func SummarizeDomains(netscape string) []DomainSummary {
	now := time.Now().Unix()

	byDomain := map[string]*DomainSummary{}
	names := map[string]map[string]bool{}

	for _, line := range strings.Split(netscape, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		// "#HttpOnly_" is part of the domain field, not a comment.
		httpOnly := strings.HasPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(line, "#") && !httpOnly {
			continue
		}

		fields := strings.Split(strings.TrimPrefix(line, "#HttpOnly_"), "\t")
		if len(fields) < 7 {
			continue
		}

		domain := strings.TrimPrefix(strings.TrimSpace(fields[0]), ".")
		if domain == "" {
			continue
		}

		entry, ok := byDomain[domain]
		if !ok {
			entry = &DomainSummary{Domain: domain}
			byDomain[domain] = entry
			names[domain] = map[string]bool{}
		}

		entry.Count++
		names[domain][fields[5]] = true

		// Normalised on the way in as well as on the way out: a file that
		// was imported before the conversion learned about Chrome's unit is
		// still on disk, and re-importing is not something to ask of anybody.
		raw, err := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
		expiry := int64(normalizeExpiry(raw))

		// The site's session cookie is the only one whose date is worth
		// reporting, so it is recorded as it goes past.
		if name, ok := KeyCookieFor(domain); ok && fields[5] == name {
			entry.KeyCookie = KeyCookie{
				Name:    name,
				Known:   true,
				Present: true,
			}
			if expiry > 0 {
				entry.KeyCookie.Expiry = time.Unix(expiry, 0)
				entry.KeyCookie.Expired = expiry < now
			}
		}

		switch {
		case err != nil || expiry == 0:
			entry.Session++
		case expiry < now:
			entry.Expired++
		default:
			at := time.Unix(expiry, 0)

			// An expiry beyond any date a browser would set is a broken
			// value, and there are plenty of them: an export that wrote
			// microseconds where seconds belong produces a date hundreds of
			// millions of years away, and showing it is worse than showing
			// nothing. It is not counted as expired either — the cookie is
			// not known to have died, only to have an unreadable date.
			if at.After(saneExpiry) {
				continue
			}

			if at.After(entry.NewestExpiry) {
				entry.NewestExpiry = at
			}
		}
	}

	out := make([]DomainSummary, 0, len(byDomain))
	for domain, entry := range byDomain {
		entry.MissingLogin = missingLoginFor(domain, names[domain])

		// A site whose session cookie is not in the file at all is signed
		// out, and that is the state worth reporting: it is why a download
		// from that site comes back as a guest.
		if name, ok := KeyCookieFor(domain); ok && !entry.KeyCookie.Present {
			entry.KeyCookie = KeyCookie{Name: name, Known: true}
		}

		out = append(out, *entry)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Domain < out[j].Domain
	})

	return out
}

// missingLoginFor is the login-cookie rule for one domain.
//
// Domains are matched the same way the rest of the package matches them: a
// subdomain belongs to its site, so "accounts.youtube.com" is judged by
// YouTube's rule and "www.bilibili.com" by Bilibili's — which has none.
func missingLoginFor(domain string, present map[string]bool) []string {
	host := strings.TrimPrefix(strings.ToLower(domain), "www.")

	rule := loginCookiesFor(host)
	if len(rule) == 0 {
		return nil
	}

	var missing []string
	for _, name := range rule {
		if !present[name] {
			missing = append(missing, name)
		}
	}

	return missing
}
