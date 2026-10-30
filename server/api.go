package server

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
)

// A download, as a caller that is not the web interface starts one.
//
// The queue and the downloader live in an internal package, because nothing
// outside the server has any business building the arguments of a yt-dlp
// invocation. A desktop application is not outside the server, though: it is
// the same program with a window on it, and it has to be able to start a
// download the same way the web interface does.
//
// So the two things a caller needs are here, and the details stay where they
// belong.

// QueueDownload puts a URL on the queue.
//
// The arguments are the ones the web interface would send: an empty list
// means the site's profile and the configuration decide, which is what a
// person pasting a URL wants.
//
// It returns the id of the download, which is what the queue row is keyed by.
func (c *Core) QueueDownload(url string, args []string) string {
	d := downloaders.NewGenericDownload(url, args)

	id := c.MDB.Set(d)
	c.MQ.Publish(d)

	return id
}

// RecordLivestream starts watching a stream and recording it.
//
// The minutes are how long to record for; zero means until the recording is
// stopped by hand. The directory is where the file is written, and an empty
// one means the configured download directory.
//
// It returns the URL, which is what the stream is known by.
func (c *Core) RecordLivestream(url string, minutes int, dir string) string {
	c.LM.AddFrom(url, minutes, "", dir)
	return url
}
