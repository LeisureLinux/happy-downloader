package task

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/marcopiovanello/yt-dlp-web-ui/v3/server/config"
	"github.com/marcopiovanello/yt-dlp-web-ui/v3/server/internal"
	"github.com/marcopiovanello/yt-dlp-web-ui/v3/server/subscription/domain"
	"github.com/robfig/cron/v3"
)

type TaskRunner interface {
	Submit(subcription *domain.Subscription) error
	Spawner(ctx context.Context)
	Recoverer()
}

type taskPair struct {
	Schedule     cron.Schedule
	Subscription *domain.Subscription
}

type CronTaskRunner struct {
	mq *internal.MessageQueue
	db *internal.MemoryDB

	tasks  chan taskPair
	errors chan error
}

func NewCronTaskRunner(mq *internal.MessageQueue, db *internal.MemoryDB) TaskRunner {
	return &CronTaskRunner{
		mq:     mq,
		db:     db,
		tasks:  make(chan taskPair),
		errors: make(chan error),
	}
}

const commandTemplate = "-I1 --flat-playlist --print webpage_url $1"

var argsSplitterRe = regexp.MustCompile(`(?mi)[^\s"']+|"([^"]*)"|'([^']*)'`)

func (t *CronTaskRunner) Submit(subcription *domain.Subscription) error {
	schedule, err := cron.ParseStandard(subcription.CronExpr)
	if err != nil {
		return err
	}

	job := taskPair{
		Schedule:     schedule,
		Subscription: subcription,
	}

	t.tasks <- job

	return nil
}

func (t *CronTaskRunner) Spawner(ctx context.Context) {
	for task := range t.tasks {
		go func() {
			for {
				slog.Info("fetching latest video for channel", slog.String("channel", task.Subscription.URL))

				fetcherParams := strings.Split(strings.Replace(commandTemplate, "$1", task.Subscription.URL, 1), " ")

				cmd := exec.CommandContext(
					ctx,
					config.Instance().DownloaderPath,
					fetcherParams...,
				)

				stdout, err := cmd.Output()
				if err != nil {
					t.errors <- err
					return
				}

				latestChannelURL := string(bytes.Trim(stdout, "\n"))

				p := &internal.Process{
					Url: latestChannelURL,
					Params: append(argsSplitterRe.FindAllString(task.Subscription.Params, 1), []string{
						"--download-archive",
						filepath.Join(config.Instance().Dir(), "archive.txt"),
					}...),
					AutoRemove: true,
				}

				t.db.Set(p)
				t.mq.Publish(p)

				nextSchedule := time.Until(task.Schedule.Next(time.Now()))

				slog.Info(
					"cron task runner next schedule",
					slog.String("url", task.Subscription.URL),
					slog.Any("duration", nextSchedule),
				)

				time.Sleep(nextSchedule)
			}
		}()
	}
}

func (t *CronTaskRunner) Recoverer() {
	panic("Unimplemented")
}
