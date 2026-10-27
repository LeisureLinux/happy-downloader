package filebrowser

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Caption files on disk.
//
// yt-dlp writes a downloaded caption track next to the video, named after the
// video with the language code and the extension appended:
//
//   Video Title.en.srt
//   Video Title.zh-Hans.srt
//
// So the tracks belonging to a file are found by looking for files whose name
// starts with the video's name and whose extension is a caption format. The
// part in between is the language.

var (
	captionExts = map[string]bool{
		".srt": true,
		".vtt": true,
		".ass": true,
		".ssa": true,
	}

	// The language code, e.g. "en" or "zh-Hans".
	languageCode = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z]{2,8})*$`)
)

// subtitlesFor finds the caption files that belong to a media file.
//
// yt-dlp writes a caption track as "<video name without extension>.<lang>.<ext>",
// so the prefix is the video's stem rather than its full name. Matching on the
// full name finds nothing at all.
//
// Only the extension and the language code are allowed to differ. Without that
// strictness a video called "Part 1" would pick up the captions of "Part 10".
func subtitlesFor(dir, name string) []SubtitleFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	stem := strings.TrimSuffix(name, filepath.Ext(name))
	prefix := stem + "."

	var found []SubtitleFile

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !captionExts[ext] {
			continue
		}

		candidate := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if !strings.HasPrefix(candidate, prefix) {
			continue
		}

		lang := strings.TrimPrefix(candidate, prefix)
		if !languageCode.MatchString(lang) {
			continue
		}

		found = append(found, SubtitleFile{
			Path:     filepath.Join(dir, e.Name()),
			Language: lang,
			Label:    languageLabel(lang),
		})
	}

	sort.Slice(found, func(i, j int) bool {
		return found[i].Language < found[j].Language
	})

	return found
}

// languageLabel turns a language code into something readable.
//
// A bare code is shown as itself, because that is what yt-dlp named the file
// and what the browser will show in its own subtitle menu.
func languageLabel(lang string) string {
	if name, ok := languageNames[lang]; ok {
		return name
	}

	if name, ok := languageNames[strings.ToLower(lang)]; ok {
		return name
	}

	return lang
}

// The subset of languages worth naming in full. The rest show their code,
// which is what the filename says anyway.
var languageNames = map[string]string{
	"en":      "English",
	"zh":      "中文",
	"zh-CN":   "中文（简体）",
	"zh-Hans": "中文（简体）",
	"zh-TW":   "中文（繁體）",
	"zh-Hant": "中文（繁體）",
	"ja":      "日本語",
	"ko":      "한국어",
	"es":      "Español",
	"fr":      "Français",
	"de":      "Deutsch",
	"it":      "Italiano",
	"pt":      "Português",
	"ru":      "Русский",
	"uk":      "Українська",
	"nl":      "Nederlands",
	"pl":      "Polski",
	"sv":      "Svenska",
	"tr":      "Türkçe",
	"ar":      "العربية",
	"hi":      "हिन्दी",
	"vi":      "Tiếng Việt",
	"th":      "ไทย",
	"id":      "Bahasa Indonesia",
}
