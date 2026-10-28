package cookies

import (
	"strings"
)

// Which site's rule applies to a cookie export.
//
// The rule that a file must carry a signed-in session is YouTube's, and it is
// expressed in the names of YouTube's cookies. Another site has other names,
// or none at all — a browser that has merely visited a site has cookies for
// it that are not a session.
//
// So the rule is asked of the site the file is for, and a file that has
// cookies for several sites is judged by the one that has a rule.

// The cookies that say a session is signed in, by site.
//
// A site that is not here has no such rule: its cookies are accepted as they
// are, because there is nothing to check them against.
var loginCookiesBySite = map[string][]string{
	"youtube.com": {"LOGIN_INFO", "HSID", "SID", "__Secure-3PSID"},
}

// The site a cookie file is for.
//
// It is read from the domains the file mentions, rather than guessed: a file
// with cookies for one site is for that site, and a file with cookies for
// several is judged by the first one that has a rule.
func siteOf(content string) string {
	for _, domain := range domainsIn(content) {
		host := strings.TrimPrefix(strings.ToLower(domain), ".")
		host = strings.TrimPrefix(host, "www.")

		for site := range loginCookiesBySite {
			if host == site || strings.HasSuffix(host, "."+site) {
				return site
			}
		}
	}

	return ""
}

// missingForSite returns the login cookies the file is missing, for the site
// the file is for.
//
// An empty answer means there is nothing to check: either the file is for a
// site with no rule, or it carries what that site needs.
func missingForSite(content string, summary Summary) []string {
	site := siteOf(content)
	if site == "" {
		return nil
	}

	wanted := loginCookiesBySite[site]

	present := make(map[string]bool, len(summary.Names))
	for _, name := range summary.Names {
		present[name] = true
	}

	var missing []string
	for _, name := range wanted {
		if !present[name] {
			missing = append(missing, name)
		}
	}

	return missing
}

// domainsIn returns the domains a Netscape cookie file mentions.
func domainsIn(content string) []string {
	var domains []string

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		domains = append(domains, fields[0])
	}

	return domains
}
