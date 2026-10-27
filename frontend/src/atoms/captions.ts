import { atom } from 'jotai'

// A caption download that is running, keyed by the file it belongs to.
//
// The job lives on the server, not in the page that asked for it: the captions
// arrive whether or not anyone is watching. So the state of a running job has
// to outlive the dialog, the file browser and even the route — otherwise
// closing the dialog loses the only thing that says "this is still going".
//
// The key is the file's path, because that is what the file browser shows and
// what the job is about. A second request for the same file replaces the
// first: the server serialises them anyway, and two "downloading" labels on
// one row would be a lie.
export type CaptionJob = {
  /** the file the captions are being fetched for */
  path: string
  /** the languages that were asked for */
  languages: string[]
  /** the server's job id, for the status poll */
  jobId: string
}

export const captionJobsState = atom<Record<string, CaptionJob>>({})

// How many caption jobs have finished.
//
// The file browser watches this to re-list the directory when the caption
// files land: they appear next to the video as new rows, and the row's chip
// has to say so. A counter rather than a list, because the browser only needs
// to know that something changed.
export const captionDoneCountState = atom(0)
