package cookies

import (
	"os"
	"strings"
	"testing"
)

// A browser's export holds the cookies of every site it has visited, so
// importing "the one with Douyin in it" is often the same file as "the one
// with YouTube in it". Writing it over the previous one dropped whichever
// site was not in the last file — and nothing said so.
func TestImportMergesRatherThanReplaces(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	youtube := "# Netscape HTTP Cookie File\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tLOGIN_INFO\tyt-value\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tHSID\tyt-hsid\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tSID\tyt-sid\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\t__Secure-3PSID\tyt-3psid\n"

	douyin := "# Netscape HTTP Cookie File\n" +
		".douyin.com\tTRUE\t/\tTRUE\t1798761600\ts_v_web_id\tdy-value\n" +
		".douyin.com\tTRUE\t/\tTRUE\t1798761600\t__ac_nonce\tdy-nonce\n"

	if _, err := store.Import([]byte(youtube)); err != nil {
		t.Fatalf("Import(youtube): %v", err)
	}

	if _, err := store.Import([]byte(douyin)); err != nil {
		t.Fatalf("Import(douyin): %v", err)
	}

	content, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}

	got := string(content)

	// Both sites are there.
	for _, want := range []string{"LOGIN_INFO", "HSID", "SID", "__Secure-3PSID", "s_v_web_id", "__ac_nonce"} {
		if !strings.Contains(got, "\t"+want+"\t") {
			t.Errorf("%s is missing from the merged file:\n%s", want, got)
		}
	}

	// And the count says so.
	cur := store.Current()
	if cur == nil {
		t.Fatal("there is no current export")
	}
	if cur.Count != 6 {
		t.Errorf("count = %d, want 6", cur.Count)
	}
}

// A cookie that is refreshed takes the new value: that is the point of
// importing it.
func TestImportReplacesARefreshedCookie(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	first := "# Netscape HTTP Cookie File\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tLOGIN_INFO\told-value\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tHSID\th\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tSID\tkeep-me\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\t__Secure-3PSID\ts\n"

	second := "# Netscape HTTP Cookie File\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tLOGIN_INFO\tnew-value\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tHSID\th\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tSID\tkeep-me\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\t__Secure-3PSID\ts\n"

	if _, err := store.Import([]byte(first)); err != nil {
		t.Fatalf("Import: %v", err)
	}

	if _, err := store.Import([]byte(second)); err != nil {
		t.Fatalf("Import: %v", err)
	}

	content, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}

	got := string(content)

	if strings.Contains(got, "old-value") {
		t.Errorf("the old value is still there:\n%s", got)
	}
	if !strings.Contains(got, "new-value") {
		t.Errorf("the new value is missing:\n%s", got)
	}
	if !strings.Contains(got, "keep-me") {
		t.Errorf("a cookie that was not in the new export was dropped:\n%s", got)
	}
}

// The rule about a signed-in session is YouTube's, and it is expressed in
// YouTube's cookies. Applying it to every file turned away a perfectly good
// Douyin or Weibo export.
func TestImportAcceptsASiteWithoutThatRule(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// A Douyin export: no LOGIN_INFO, and that is fine.
	douyin := "# Netscape HTTP Cookie File\n" +
		".douyin.com\tTRUE\t/\tTRUE\t1798761600\ts_v_web_id\tdy-value\n"

	if _, err := store.Import([]byte(douyin)); err != nil {
		t.Fatalf("a Douyin export was refused: %v", err)
	}

	// A YouTube export that is really a guest session is still refused.
	guest := "# Netscape HTTP Cookie File\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1798761600\tPREF\ttz=UTC\n"

	store2 := NewStore(t.TempDir())
	if _, err := store2.Import([]byte(guest)); err == nil {
		t.Error("a guest session was accepted for YouTube")
	}
}

// The httpOnly prefix is what makes curl and some tools skip a line, and
// skipping a login cookie is what turns a signed-in session into a guest one.
func TestMergeKeepsTheHttpOnlyPrefix(t *testing.T) {
	existing := "# Netscape HTTP Cookie File\n" +
		"#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t1798761600\tLOGIN_INFO\tvalue\n"

	incoming := "# Netscape HTTP Cookie File\n" +
		".douyin.com\tTRUE\t/\tTRUE\t1798761600\ts_v_web_id\tdy\n"

	merged, _, _ := mergeCookies(existing, incoming)

	if !strings.Contains(merged, "#HttpOnly_.youtube.com") {
		t.Errorf("the httpOnly prefix was lost:\n%s", merged)
	}
	if !strings.Contains(merged, "s_v_web_id") {
		t.Errorf("the new cookie is missing:\n%s", merged)
	}
}
