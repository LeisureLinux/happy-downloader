package rpc

import (
	"reflect"
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
)

func TestStripOutputArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"removes the resolved output path",
			[]string{"-f", "bestaudio/best", "-o", "/media/downloads/%(title)s.%(ext)s"},
			[]string{"-f", "bestaudio/best"},
		},
		{
			"removes the long form",
			[]string{"--output", "/tmp/a.mp4", "--extract-audio"},
			[]string{"--extract-audio"},
		},
		{
			"leaves a clean request alone",
			[]string{"-f", "bestvideo+bestaudio", "--merge-output-format", "mkv"},
			[]string{"-f", "bestvideo+bestaudio", "--merge-output-format", "mkv"},
		},
		{
			"a value that merely looks like a path survives",
			[]string{"-P", "/tmp"},
			[]string{"-P", "/tmp"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := downloaders.StripOutputArgs(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}
