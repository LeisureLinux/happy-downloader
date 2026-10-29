package rpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/formats"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/livestream"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
	"github.com/LeisureLinux/happy-downloader/v4/server/playlist"
	"github.com/LeisureLinux/happy-downloader/v4/server/rest"
	"github.com/LeisureLinux/happy-downloader/v4/server/sys"
	"github.com/LeisureLinux/happy-downloader/v4/server/updater"
)

type Service struct {
	db *kv.Store
	mq *queue.MessageQueue
	lm *livestream.Monitor
}

type Running []internal.ProcessSnapshot
type Pending []string

type NoArgs struct{}

// Exec spawns a Process.
// The result of the execution is the newly spawned process Id.
func (s *Service) Exec(args internal.DownloadRequest, result *string) error {
	d := downloaders.NewGenericDownload(args.URL, args.Params)
	d.SetOutput(internal.DownloadOutput{
		Path:     args.Path,
		Filename: args.Rename,
	})
	d.SetFromIP(args.FromIP)

	s.db.Set(d)
	s.mq.Publish(d)

	*result = d.GetId()
	return nil
}

// Exec spawns a Process.
// The result of the execution is the newly spawned process Id.
func (s *Service) ExecPlaylist(args internal.DownloadRequest, result *string) error {
	err := playlist.PlaylistDetect(args, s.mq, s.db)
	if err != nil {
		return err
	}

	*result = ""
	return nil
}

// TODO: docs
func (s *Service) ExecLivestream(args internal.DownloadRequest, result *string) error {
	s.lm.AddFrom(args.URL, args.RecordMinutes, args.FromIP, args.Path)

	*result = args.URL
	return nil
}

// TODO: docs
func (s *Service) ProgressLivestream(args NoArgs, result *livestream.LiveStreamStatus) error {
	*result = s.lm.Status()
	return nil
}

// TODO: docs
func (s *Service) KillLivestream(args string, result *struct{}) error {
	slog.Info("killing livestream", slog.String("url", args))

	err := s.lm.Remove(args)
	if err != nil {
		slog.Error("failed killing livestream", slog.String("url", args), slog.Any("err", err))
		return err
	}

	return nil
}

// TODO: docs
func (s *Service) KillAllLivestream(args NoArgs, result *struct{}) error {
	return s.lm.RemoveAll()
}

// Progess retrieves the Progress of a specific Process given its Id
func (s *Service) Progess(args internal.DownloadRequest, progress *internal.DownloadProgress) error {
	dl, err := s.db.Get(args.Id)
	if err != nil {
		return err
	}

	*progress = dl.Status().Progress
	return nil
}

// Progess retrieves available format for a given resource
func (s *Service) Formats(args internal.DownloadRequest, meta *formats.Metadata) error {
	var err error

	metadata, err := formats.ParseURL(args.URL)
	if err != nil && metadata == nil {
		return err
	}

	if metadata.IsPlaylist() {
		go playlist.PlaylistDetect(args, s.mq, s.db)
	}

	*meta = *metadata
	return nil
}

// Pending retrieves a slice of all Pending/Running processes ids
func (s *Service) Pending(args NoArgs, pending *Pending) error {
	*pending = *s.db.Keys()
	return nil
}

// Running retrieves a slice of all Processes progress
func (s *Service) Running(args NoArgs, running *Running) error {
	*running = *s.db.All()
	return nil
}

// Kill kills a process given its id and remove it from the memoryDB
func (s *Service) Kill(args string, killed *string) error {
	slog.Info("Trying killing process with id", slog.String("id", args))

	download, err := s.db.Get(args)
	if err != nil {
		return err
	}

	if download == nil {
		return errors.New("nil process")
	}

	s.db.Delete(download.GetId())

	if err := download.Stop(); err != nil {
		slog.Info("failed killing process", slog.String("id", download.GetId()), slog.Any("err", err))
		return err
	}

	slog.Info("succesfully killed process", slog.String("id", download.GetId()))

	return nil
}

// KillAll kills all process unconditionally and removes them from
// the memory db
func (s *Service) KillAll(args NoArgs, killed *string) error {
	slog.Info("Killing all spawned processes")

	var (
		keys       = s.db.Keys()
		removeFunc = func(d downloaders.Downloader) error {
			defer s.db.Delete(d.GetId())
			return d.Stop()
		}
	)

	for _, key := range *keys {
		dl, err := s.db.Get(key)
		if err != nil {
			return err
		}

		if dl == nil {
			s.db.Delete(key)
			continue
		}

		if err := removeFunc(dl); err != nil {
			slog.Info(
				"failed killing process",
				slog.String("id", dl.GetId()),
				slog.Any("err", err),
			)
			continue
		}

		slog.Info("succesfully killed process", slog.String("id", dl.GetId()))
	}

	return nil
}

// Retry re-queues a download that failed, reusing the arguments it was created
// with. The old entry is replaced rather than kept alongside, so a retry does
// not leave two rows for one URL with different outcomes.
func (s *Service) Retry(args string, result *string) error {
	old, err := s.db.Get(args)
	if err != nil {
		return err
	}

	if old == nil {
		return errors.New("nil process")
	}

	snapshot := old.Status()
	url := old.GetUrl()

	if url == "" {
		return errors.New("this entry has no URL recorded, so it cannot be retried")
	}

	s.db.Delete(args)

	d := downloaders.NewGenericDownload(url, downloaders.StripOutputArgs(snapshot.Params))
	d.SetOutput(internal.DownloadOutput{
		Path:     snapshot.Output.Path,
		Filename: snapshot.Output.Filename,
	})

	s.db.Set(d)
	s.mq.Publish(d)

	slog.Info("retrying download",
		slog.String("old", args),
		slog.String("new", d.GetId()),
		slog.String("url", url),
	)

	*result = d.GetId()

	return nil
}

// StartRecording starts a recording that was stopped on purpose.
//
// It is not a retry: nothing failed, and the files already written are the
// point of the exercise. The same row runs again, from the live edge, and
// writes a new file for the new segment.
func (s *Service) StartRecording(args string, result *string) error {
	download, err := s.db.Get(args)
	if err != nil {
		return err
	}

	if download == nil {
		return errors.New("nil process")
	}

	recorder, ok := download.(*downloaders.LiveStreamDownloader)
	if !ok {
		return errors.New("this entry is not a recording")
	}

	recorder.PrepareRestart()
	s.mq.Publish(recorder)

	slog.Info("starting the recording again", slog.String("id", args))

	*result = args
	return nil
}

// Pause stops a running download but keeps it in the queue.
//
// The entry stays where it is, with the progress it had, and the button on
// its row becomes Resume. The partial file yt-dlp left behind is what makes
// the resume cheap: yt-dlp continues a partial download by default.
func (s *Service) Pause(args string, paused *string) error {
	slog.Info("pausing download", slog.String("id", args))

	download, err := s.db.Get(args)
	if err != nil {
		return err
	}

	if download == nil {
		return errors.New("nil process")
	}

	if err := download.Stop(); err != nil {
		slog.Info("failed pausing download",
			slog.String("id", download.GetId()),
			slog.Any("err", err))
		return err
	}

	*paused = args

	return nil
}

// Resume starts a paused download again.
//
// It re-queues the same URL with the same arguments, so the resumed download
// is the same one that was paused — and because yt-dlp keeps the partial
// file, it continues rather than starting over. The old entry is replaced
// rather than kept alongside, so there is one row for one download.
//
// A download that is not paused is left alone: resuming something that is
// already running would start a second copy of it.
func (s *Service) Resume(args string, resumed *string) error {
	slog.Info("resuming download", slog.String("id", args))

	old, err := s.db.Get(args)
	if err != nil {
		return err
	}

	if old == nil {
		return errors.New("nil process")
	}

	snapshot := old.Status()

	if snapshot.Progress.Status != internal.StatusPaused {
		return errors.New("this download is not paused")
	}

	url := old.GetUrl()
	if url == "" {
		return errors.New("this entry has no URL recorded, so it cannot be resumed")
	}

	s.db.Delete(args)

	d := downloaders.NewGenericDownload(url, downloaders.StripOutputArgs(snapshot.Params))
	d.SetOutput(internal.DownloadOutput{
		Path:     snapshot.Output.Path,
		Filename: snapshot.Output.Filename,
	})

	s.db.Set(d)
	s.mq.Publish(d)

	slog.Info("resumed download",
		slog.String("old", args),
		slog.String("new", d.GetId()),
		slog.String("url", url),
	)

	*resumed = d.GetId()

	return nil
}

// Remove a process from the db rendering it unusable if active
func (s *Service) Clear(args string, killed *string) error {
	slog.Info("Clearing process with id", slog.String("id", args))
	s.db.Delete(args)
	return nil
}

// Removes completed processes
func (s *Service) ClearCompleted(cleared *string) error {
	var (
		keys       = s.db.Keys()
		removeFunc = func(d downloaders.Downloader) error {
			defer s.db.Delete(d.GetId())

			if !d.IsCompleted() {
				return nil
			}

			return d.Stop()
		}
	)

	for _, key := range *keys {
		proc, err := s.db.Get(key)
		if err != nil {
			return err
		}

		if err := removeFunc(proc); err != nil {
			return err
		}
	}

	return nil
}

// FreeSpace gets the available from package sys util
func (s *Service) FreeSpace(args NoArgs, free *uint64) error {
	freeSpace, err := sys.FreeSpace()
	if err != nil {
		return err
	}

	*free = freeSpace
	return err
}

// Return a flattned tree of the download directory
func (s *Service) DirectoryTree(args NoArgs, tree *[]string) error {
	dfsTree, err := sys.DirectoryTree()

	if err != nil {
		*tree = nil
		return err
	}

	if dfsTree != nil {
		*tree = *dfsTree
	}

	return nil
}

// CheckYtdlpUpdate reports whether a newer yt-dlp is available.
//
// The answer is cached for a while, because it is a network call and the
// footer asks for it every time the page loads. A day is enough: yt-dlp
// breaks YouTube every few weeks, and a release that came out an hour ago
// can wait until tomorrow.
func (s *Service) CheckYtdlpUpdate(args NoArgs, result *updater.Status) error {
	updater.CheckOnce(context.Background(), result)
	return nil
}

// UpdateYtdlp installs a newer yt-dlp.
//
// The binary is replaced rather than upgraded in place: a yt-dlp installed by
// a package manager refuses to update itself, and says so. The GitHub binary
// is downloaded and put in place of the current one, atomically, so a download
// starting at that moment finds either the old binary or the new one.
func (s *Service) UpdateYtdlp(args NoArgs, updated *bool) error {
	slog.Info("updating the yt-dlp binary")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := updater.Update(ctx); err != nil {
		slog.Error("could not update yt-dlp", slog.Any("err", err))
		*updated = false
		return err
	}

	// The cached answers are about the version that was just replaced.
	updater.Invalidate()
	rest.ForgetYtdlpVersion()

	*updated = true
	slog.Info("yt-dlp updated")

	return nil
}

// Updates the yt-dlp binary using its builtin function
func (s *Service) UpdateExecutable(args NoArgs, updated *bool) error {
	slog.Info("Updating yt-dlp executable to the latest release")

	if err := updater.UpdateExecutable(); err != nil {
		slog.Error("Failed updating yt-dlp")
		*updated = false
		return err
	}

	*updated = true
	slog.Info("Succesfully updated yt-dlp")

	return nil
}
