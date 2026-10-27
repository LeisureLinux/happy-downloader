import { useEffect, useRef } from 'react'
import { useAtom, useAtomValue, useSetAtom } from 'jotai'
import { captionDoneCountState, captionJobsState } from '../atoms/captions'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'

// Watches the caption jobs that are running and reports how they ended.
//
// The dialog that started a job closes as soon as the request is accepted, so
// the poll cannot live there: the component is unmounted, and the work
// disappears from the page even though the server is still doing it. This
// lives at the top of the app instead, and the file browser reads the same
// atom to say "n captions downloading" next to the file's name.
//
// A job is removed from the atom when it finishes, so the file browser shows
// the label only while there is something to wait for. After that the
// subtitles appear in the list and the row looks like every other row.
export const CaptionJobWatcher: React.FC = () => {
  const serverAddr = useAtomValue(serverURL)
  const [jobs, setJobs] = useAtom(captionJobsState)
  const setDoneCount = useSetAtom(captionDoneCountState)
  const { pushMessage } = useToast()
  const { i18n } = useI18n()

  // pushMessage is a new function on every render, and the translator is a
  // singleton whose strings change with the language. Keeping both in refs
  // stops the poll from restarting every time anything re-renders.
  const notifyRef = useRef(pushMessage)
  notifyRef.current = pushMessage

  const i18nRef = useRef(i18n)
  i18nRef.current = i18n

  // The job list in a ref, so the effect below can read the current jobs
  // without depending on the map's identity. The effect restarts when the set
  // of job ids changes, which is exactly when a new poll is wanted.
  const jobsRef = useRef(jobs)
  jobsRef.current = jobs

  const keys = Object.keys(jobs).sort().join(',')

  useEffect(() => {
    if (!keys) return

    let cancelled = false
    const ids = keys.split(',')

    const finish = (path: string, message: string, severity: 'success' | 'info' | 'error') => {
      if (cancelled) return
      notifyRef.current(message, severity)
      setJobs(prev => {
        const next = { ...prev }
        delete next[path]
        return next
      })
    }

    const poll = async () => {
      for (const path of ids) {
        if (cancelled) return

        const job = jobsRef.current[path]
        if (!job) continue

        try {
          const res = await fetch(`${serverAddr}/api/v1/captions/status/${job.jobId}`)
          if (!res.ok) continue

          const state = await res.json()

          if (state.state === 'failed') {
            finish(path, i18nRef.current.t('captionsFailed'), 'error')
            return
          }

          if (state.state === 'succeeded') {
            const written = state.written?.length ?? 0
            const skipped = Object.keys(state.skipped ?? {}).length

            finish(
              path,
              written > 0
                ? i18nRef.current.t('captionsDownloaded').replace('{count}', String(written))
                : i18nRef.current.t('captionsNone'),
              written > 0 ? 'success' : 'info'
            )

            // The caption files are on disk now, so the directory has changed.
            setDoneCount(n => n + 1)

            if (skipped > 0) {
              finish(
                path,
                i18nRef.current.t('captionsSkipped').replace('{count}', String(skipped)),
                'info'
              )
            }
            return
          }
        } catch {
          // A dropped poll is not a failed job; the server is still working.
        }
      }
    }

    const timer = setInterval(() => { void poll() }, 2000)

    // One immediate poll, so a job that finished while nobody was looking is
    // reported straight away rather than two seconds later.
    void poll()

    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [keys, serverAddr, setJobs, setDoneCount])

  return null
}

export default CaptionJobWatcher
