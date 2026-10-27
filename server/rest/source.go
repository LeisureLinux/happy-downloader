package rest

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// Finding a file's source from the metadata inside it.
//
// yt-dlp's --embed-metadata writes the page's URL into the container's comment
// field, and the title alongside it. So a file that was downloaded before the
// server started remembering — or one that was put there by hand with the same
// tool — still knows where it came from.
//
// Reading it costs a process spawn, so it is a fallback rather than the first
// choice: the server's own index is faster and does not depend on the file
// having been tagged at all.

// sourceFromMetadata reads the source URL out of a media file.
func sourceFromMetadata(path string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=comment,description,title",
		"-of", "json",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", false
	}

	var parsed struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", false
	}

	return sourceFromTags(parsed.Format.Tags)
}

// sourceFromTags picks the source URL out of a tag set.
//
// The comment field is where yt-dlp puts it. The description is checked too,
// because some containers carry it there instead, and a description that
// starts with a URL is still a source.
func sourceFromTags(tags map[string]string) (string, bool) {
	for _, key := range []string{"comment", "description"} {
		value := strings.TrimSpace(tags[key])
		if value == "" {
			continue
		}

		if url, ok := urlInText(value); ok {
			return url, true
		}
	}

	return "", false
}

// urlInText finds the first http URL in a piece of text.
func urlInText(text string) (string, bool) {
	const scheme = "http"

	index := strings.Index(text, scheme)
	if index == -1 {
		return "", false
	}

	end := index
	for end < len(text) && !isURLTerminator(text[end]) {
		end++
	}

	// Trailing punctuation belongs to the sentence, not the URL. A URL that
	// ends in a period is not a URL anyone can fetch.
	url := strings.TrimRight(text[index:end], ".,;:)]}\"'")

	if url == scheme {
		return "", false
	}

	return url, true
}

func isURLTerminator(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '<', '>', '"', '\'':
		return true
	}
	return false
}
