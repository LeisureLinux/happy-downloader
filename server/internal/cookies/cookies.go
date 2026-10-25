// Package cookies converts a browser cookie export into the Netscape
// cookies.txt format that yt-dlp, curl and wget read.
//
// Browser extensions export one of several shapes. The JSON array produced by
// Cookie-Editor, EditThisCookie and Get cookies.txt LOCALLY carries every
// attribute a cookie has, including httpOnly; the Netscape file carries the
// same fields but is easy to truncate by hand. Anything else is either a
// name=value header string or a bare key-value map, neither of which records a
// domain, a path or an expiry.
//
// The distinction matters on a headless host: the export arrives as a file
// with no browser to re-export from, so the format has to be recognised from
// the content.
package cookies

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Format identifies the shape an export was recognised as.
type Format string

const (
	// FormatNetscape is the tab-separated cookies.txt layout.
	FormatNetscape Format = "netscape"
	// FormatJSON is the array-of-objects shape used by browser extensions.
	FormatJSON Format = "json"
	// FormatHeader is a single "name=value; name=value" line.
	FormatHeader Format = "header"
	// FormatUnknown means the content could not be recognised.
	FormatUnknown Format = ""
)

// Cookie is one entry, holding the union of the fields every format uses.
type Cookie struct {
	Domain         string  `json:"domain"`
	Path           string  `json:"path"`
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	ExpirationDate float64 `json:"expirationDate"`
	Secure         bool    `json:"secure"`
	HttpOnly       bool    `json:"httpOnly"`
	// Session cookies carry no expiry; cookies.txt records them as 0.
	Session bool `json:"session"`
}

// Detect reports which format the content is in, without converting.
func Detect(content []byte) Format {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return FormatUnknown
	}

	switch trimmed[0] {
	case '[', '{':
		if json.Valid(trimmed) {
			return FormatJSON
		}
		return FormatUnknown
	}

	if bytes.HasPrefix(trimmed, []byte("#")) {
		return FormatNetscape
	}

	// "name=value" with no tabs cannot be Netscape, which is tab-separated.
	if bytes.Contains(trimmed, []byte("=")) && !bytes.Contains(trimmed, []byte("\t")) {
		return FormatHeader
	}

	if bytes.Contains(trimmed, []byte("\t")) {
		return FormatNetscape
	}

	return FormatUnknown
}

// Convert rewrites content into Netscape cookies.txt.
//
// The input format is detected from the content, so a JSON export from a
// browser extension and a hand-made cookies.txt both work without the caller
// having to know which it received.
func Convert(content []byte) (string, Format, error) {
	switch Detect(content) {
	case FormatNetscape:
		return string(content), FormatNetscape, nil
	case FormatJSON:
		n, err := JSONToNetscape(content)
		return n, FormatJSON, err
	case FormatHeader:
		n, err := HeaderToNetscape(content)
		return n, FormatHeader, err
	default:
		return "", FormatUnknown, fmt.Errorf("unrecognised cookie export format")
	}
}

// JSONToNetscape converts the extension/Puppeteer JSON shape to Netscape.
//
// Two details decide whether the result is accepted:
//
//   - HttpOnly cookies are written with the "#HttpOnly_" domain prefix. Without
//     it curl and some tools skip the line, and the session cookies that matter
//     most on YouTube are exactly the HttpOnly ones.
//   - A session cookie (no expiry) is written with expiry 0. Writing an invented
//     timestamp for a cookie the browser never expires makes it look stale once
//     that time passes.
func JSONToNetscape(content []byte) (string, error) {
	var raw any
	if err := json.Unmarshal(bytes.TrimSpace(content), &raw); err != nil {
		return "", fmt.Errorf("invalid JSON cookie export: %w", err)
	}

	list, err := extractList(raw)
	if err != nil {
		return "", err
	}

	if len(list) == 0 {
		return "", fmt.Errorf("cookie export contains no entries")
	}

	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	b.WriteString("# Generated from a JSON cookie export\n\n")

	for _, c := range list {
		line, err := netscapeLine(c)
		if err != nil {
			// One malformed entry should not discard the whole file.
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// extractList normalises the wrapper shapes into a slice of cookies.
func extractList(raw any) ([]Cookie, error) {
	switch v := raw.(type) {
	case []any:
		var out []Cookie
		for _, item := range v {
			c, err := decodeCookie(item)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil

	case map[string]any:
		for _, key := range []string{"cookies", "Cookies", "data", "items"} {
			if inner, ok := v[key]; ok {
				return extractList(inner)
			}
		}
		c, err := decodeCookie(v)
		if err != nil {
			return nil, err
		}
		return []Cookie{c}, nil

	default:
		return nil, fmt.Errorf("unexpected JSON structure %T", raw)
	}
}

func decodeCookie(item any) (Cookie, error) {
	m, ok := item.(map[string]any)
	if !ok {
		return Cookie{}, fmt.Errorf("cookie entry is %T, want object", item)
	}

	c := Cookie{
		Domain:   asString(m["domain"]),
		Path:     asString(m["path"]),
		Name:     asString(m["name"]),
		Value:    asString(m["value"]),
		Secure:   asBool(m["secure"]),
		HttpOnly: asBool(m["httpOnly"]) || asBool(m["httponly"]),
		Session:  asBool(m["session"]),
	}

	// Expiry arrives under one of several names depending on the exporter:
	// Chrome's extension API and Puppeteer use "expirationDate", Firefox and
	// Cookie-Editor use "expires". Treating a present expiry as absent because
	// the field was named differently turns every cookie into a session cookie.
	for _, key := range []string{"expirationDate", "expires", "expiry"} {
		if exp, ok := m[key]; ok && exp != nil {
			c.ExpirationDate = asFloat(exp)
			c.Session = c.Session || c.ExpirationDate == 0
			break
		}
	}

	if c.ExpirationDate == 0 {
		c.Session = true
	}

	if c.Name == "" {
		return Cookie{}, fmt.Errorf("cookie entry has no name")
	}
	if c.Domain == "" {
		return Cookie{}, fmt.Errorf("cookie %q has no domain", c.Name)
	}

	return c, nil
}

func netscapeLine(c Cookie) (string, error) {
	domain := c.Domain

	includeSubdomains := "FALSE"
	if strings.HasPrefix(domain, ".") {
		includeSubdomains = "TRUE"
	}

	prefix := ""
	if c.HttpOnly {
		prefix = "#HttpOnly_"
	}

	path := c.Path
	if path == "" {
		path = "/"
	}

	expiry := int64(0)
	if !c.Session && c.ExpirationDate > 0 {
		expiry = int64(c.ExpirationDate)
	}

	secure := "FALSE"
	if c.Secure {
		secure = "TRUE"
	}

	return strings.Join([]string{
		prefix + domain,
		includeSubdomains,
		path,
		secure,
		strconv.FormatInt(expiry, 10),
		c.Name,
		c.Value,
	}, "\t"), nil
}

// HeaderToNetscape converts a "name=value; name=value" line.
//
// The result is minimal by necessity: the header form carries no domain, no
// path and no expiry, so entries are written against .youtube.com with expiry
// 0. It is better than nothing, but a JSON export is always preferable.
func HeaderToNetscape(content []byte) (string, error) {
	line := strings.TrimSpace(string(content))
	line = strings.TrimPrefix(line, "Cookie:")
	line = strings.TrimSpace(line)

	pairs := strings.Split(line, ";")

	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	b.WriteString("# Generated from a Cookie header string\n")
	b.WriteString("# Domain and path are placeholders: the header form carries neither\n\n")

	written := 0
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		name, value, ok := strings.Cut(p, "=")
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}

		b.WriteString(strings.Join([]string{
			".youtube.com", "TRUE", "/", "TRUE", "0",
			strings.TrimSpace(name), strings.TrimSpace(value),
		}, "\t"))
		b.WriteByte('\n')
		written++
	}

	if written == 0 {
		return "", fmt.Errorf("no usable name=value pairs in cookie header")
	}

	return b.String(), nil
}

// Summary describes a converted file for logging and for the status endpoint,
// without echoing any value.
type Summary struct {
	Format   Format
	Total    int
	HttpOnly int
	Secure   int
	Session  int
	// Expired counts entries whose expiry has already passed.
	Expired int
	Names   []string
}

// Summarize reports what a Netscape file contains. Cookie values are never
// included: this goes to the log and to the HTTP response, and a cookie value
// is a live credential.
func Summarize(netscape string) Summary {
	s := Summary{Format: FormatNetscape}

	now := time.Now().Unix()

	for _, line := range strings.Split(netscape, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		// A "#HttpOnly_" line is a cookie, not a comment; only a plain "#"
		// line is a comment. Spell the precedence out rather than rely on it.
		httpOnly := strings.HasPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(line, "#") && !httpOnly {
			continue
		}

		body := strings.TrimPrefix(line, "#HttpOnly_")
		fields := strings.Split(body, "\t")
		if len(fields) < 7 {
			continue
		}

		s.Total++
		if httpOnly {
			s.HttpOnly++
		}
		if fields[3] == "TRUE" {
			s.Secure++
		}

		expiry, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil || expiry == 0 {
			s.Session++
		} else if expiry < now {
			s.Expired++
		}

		s.Names = append(s.Names, fields[5])
	}

	sort.Strings(s.Names)
	return s
}

// Required reports the YouTube login cookies a file is missing.
//
// YouTube treats a session as logged-in only when these are present. An export
// carrying the __Secure-* family but lacking LOGIN_INFO is served as a guest,
// which is what turns into "Sign in to confirm you're not a bot" on videos
// that need an account.
func Required(names []string) []string {
	present := make(map[string]bool, len(names))
	for _, n := range names {
		present[n] = true
	}

	want := []string{"LOGIN_INFO", "HSID", "SID", "__Secure-3PSID"}

	var missing []string
	for _, w := range want {
		if !present[w] {
			missing = append(missing, w)
		}
	}
	return missing
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(strings.ToLower(strings.TrimSpace(t)))
		return b
	case float64:
		return t != 0
	default:
		return false
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	case int:
		return float64(t)
	default:
		return 0
	}
}
