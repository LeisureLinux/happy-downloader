import { atom } from 'jotai'
import { activeDownloadsState } from './downloads'

export const loadingAtom = atom(true)

export const totalDownloadSpeedState = atom<number>((get) =>
  get(activeDownloadsState)
    .map(d => d.progress.speed)
    .reduce((curr, next) => curr + next, 0)
)

// Whether the log panel is open. A toggle rather than a route: reading the log
// is done while watching a download, and a page would hide the download list.
export const logPanelOpenAtom = atom(false)

// Settings opened as a panel over the page rather than a route. Sitting above
// the current view matters here: most settings are changed while watching what
// they do to a download in progress.
export const settingsPanelOpenAtom = atom(false)
