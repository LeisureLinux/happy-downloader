package cookies

import (
	"strings"
	"time"
)

// Merging a new export into the one already imported.
//
// The cookies of a browser export are per site, and the export holds every
// site the browser has been to. So importing "the one with Douyin in it" is
// often the same file as "the one with YouTube in it", and writing it over
// the previous one dropped whichever site was not in the last file.
//
// A cookie of the same name and domain takes the new value — that is the
// point of importing, since a refreshed cookie is the one that works — and a
// cookie the new file does not mention is kept.

// mergeCookies adds one Netscape cookie file to another.
//
// It returns the merged file, how many cookies were added, and how many
// took a new value.
func mergeCookies(existing, incoming string) (string, int, int) {
	if strings.TrimSpace(existing) == "" {
		return incoming, 0, 0
	}

	// The key is what makes a cookie the same cookie: the domain, the path
	// and the name. The value is not part of it — that is what gets
	// replaced.
	type cookieKey struct {
		domain string
		path   string
		name   string
	}

	// The domain as the key sees it: without the "#HttpOnly_" prefix, which
	// is part of the line rather than of the domain.
	domainOf := func(field string) string {
		return strings.TrimPrefix(field, "#HttpOnly_")
	}

	merged := map[cookieKey][]string{}
	var order []cookieKey

	read := func(content string, replace bool) {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				continue
			}

			// A comment is a comment, except for the "#HttpOnly_" prefix,
			// which is part of the domain.
			if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_") {
				continue
			}

			fields := strings.Split(line, "\t")
			if len(fields) < 7 {
				continue
			}

			// The key has the domain without the prefix, so that a cookie
			// written both ways is the same cookie. The line keeps the
			// prefix — that is what the format says.
			key := cookieKey{
				domain: domainOf(fields[0]),
				path:   fields[2],
				name:   fields[5],
			}

			_, seen := merged[key]
			if !seen {
				order = append(order, key)
			}

			if seen && !replace {
				// The cookie is already there and this pass does not
				// replace it.
				continue
			}

			merged[key] = fields
		}
	}

	// What was there before, so that a cookie that took a new value can be
	// told from one that was added.
	before := map[cookieKey]bool{}

	read(existing, false)
	for _, key := range order {
		before[key] = true
	}

	read(incoming, true)

	added := 0
	replaced := 0

	for _, key := range order {
		if before[key] {
			replaced++
		} else {
			added++
		}
	}

	var out strings.Builder

	// The header every Netscape file has.
	out.WriteString("# Netscape HTTP Cookie File\n")
	out.WriteString("# Imported from a browser export by Happy Downloader\n\n")

	for _, key := range order {
		fields := merged[key]

		// The "#HttpOnly_" prefix is part of the line, and it is what makes
		// curl and some tools skip the line — which is what turned a
		// signed-in session into a guest one. It is kept.
		//
		// It was thrown away when the key was built, because the key wants
		// the domain without it. The key and the line are different things:
		// the cookie is the same whichever way it is written down.
		out.WriteString(strings.Join(fields, "\t"))
		out.WriteString("\n")
	}

	return out.String(), added, replaced
}

// The expiry of the cookies in a Netscape file, counted.
//
// The cookies of a site that is not signed in are usually short-lived: a
// browser's identifier for a visitor expires in days, and the ones that mark
// a machine expire in hours. That is worth knowing, because it is the
// difference between an export that is good for a week and one that is good
// for an afternoon.
func ExpiryOf(content string) (expired int, session int, longest time.Duration) {
	now := time.Now()

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		var seconds int64
		if _, err := fmtSscan(fields[4], &seconds); err != nil {
			// Zero means a session cookie: it lasts until the browser
			// closes.
			session++
			continue
		}

		if seconds == 0 {
			session++
			continue
		}

		left := time.Duration(seconds)*time.Second - now.Sub(now)
		if seconds > now.Unix() {
			remaining := time.Duration(seconds-now.Unix()) * time.Second
			if remaining > longest {
				longest = remaining
			}
		} else {
			expired++
		}

		_ = left
	}

	return expired, session, longest
}

// fmtSscan parses an integer, without pulling in the fmt package for one
// call.
func fmtSscan(s string, out *int64) (int, error) {
	if s == "" {
		return 0, errEmpty
	}

	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errNotANumber
		}
		n = n*10 + int64(r-'0')
	}

	*out = n
	return 1, nil
}

var (
	errEmpty      = errorString("the value is empty")
	errNotANumber = errorString("the value is not a number")
)

type errorString string

func (e errorString) Error() string { return string(e) }
