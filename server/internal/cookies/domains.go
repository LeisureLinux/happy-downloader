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
	MissingLogin []string
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

		expiry, err := strconv.ParseInt(fields[4], 10, 64)
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

	var rule []string
	for site, want := range loginCookiesBySite {
		if host == site || strings.HasSuffix(host, "."+site) {
			rule = want
			break
		}
	}

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
