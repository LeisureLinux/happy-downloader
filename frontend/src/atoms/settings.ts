import { pipe } from 'fp-ts/lib/function'
import { matchW } from 'fp-ts/lib/TaskEither'
import { ffetch } from '../lib/httpClient'
import { prefersDarkMode } from '../utils'
import { atomWithStorage } from 'jotai/utils'
import { atom } from 'jotai'

export const languages = [
  'catalan',
  'chinese',
  'chinese-traditional',
  'english',
  'french',
  'german',
  'italian',
  'japanese',
  'korean',
  'polish',
  'portuguese-br',
  'russian',
  'spanish',
  'swedish',
  'ukrainian',
  'hungarian'
] as const

export type Language = (typeof languages)[number]

// Each language named in itself. A user looking for their own language scans
// for the word they recognise, not for its English name — "Deutsch" is findable
// to a German speaker, "German" less so.
export const languageNames: Record<Language, string> = {
  catalan: 'Català',
  chinese: '简体中文',
  'chinese-traditional': '繁體中文',
  english: 'English',
  french: 'Français',
  german: 'Deutsch',
  hungarian: 'Magyar',
  italian: 'Italiano',
  japanese: '日本語',
  korean: '한국어',
  polish: 'Polski',
  'portuguese-br': 'Português (BR)',
  russian: 'Русский',
  spanish: 'Español',
  swedish: 'Svenska',
  ukrainian: 'Українська',
}

// The YouTube caption code for each interface language.
//
// The two are not the same thing: YouTube uses "zh-Hans" and "zh-Hant" where
// the interface distinguishes "chinese" and "chinese-traditional", and it
// uses "pt" where the interface says "portuguese-br". The caption request
// needs YouTube's spelling, so the mapping is explicit rather than guessed.
export const youtubeCaptionLang: Record<Language, string> = {
  catalan: 'ca',
  chinese: 'zh-Hans',
  'chinese-traditional': 'zh-Hant',
  english: 'en',
  french: 'fr',
  german: 'de',
  hungarian: 'hu',
  italian: 'it',
  japanese: 'ja',
  korean: 'ko',
  polish: 'pl',
  'portuguese-br': 'pt',
  russian: 'ru',
  spanish: 'es',
  swedish: 'sv',
  ukrainian: 'uk',
}

export type Theme = 'light' | 'dark' | 'system'
export type ThemeNarrowed = 'light' | 'dark'

export const accents = ['default', 'red', 'purple', 'orange'] as const
export type Accent = (typeof accents)[number]

export interface SettingsState {
  serverAddr: string
  serverPort: number
  language: Language
  theme: ThemeNarrowed
  accent: Accent
  cliArgs: string
  formatSelection: boolean
  fileRenaming: boolean
  autoFileExtension: boolean
  pathOverriding: boolean
  enableCustomArgs: boolean
  servedFromReverseProxy: boolean
}

export const languageState = atomWithStorage<Language>(
  'language',
  localStorage.getItem('language') as Language || 'english'
)

export const themeState = atomWithStorage<Theme>(
  'theme',
  localStorage.getItem('theme') as Theme || 'system'
)

export const serverAddressState = atomWithStorage<string>(
  'server-addr',
  localStorage.getItem('server-addr') || window.location.hostname
)

export const serverPortState = atomWithStorage<number>(
  'server-port',
  Number(localStorage.getItem('server-port')) || Number(window.location.port)
)

export const latestCliArgumentsState = atomWithStorage<string>(
  'cli-args',
  localStorage.getItem('cli-args') || '--no-mtime'
)

export const formatSelectionState = atomWithStorage(
  'format-selection',
  localStorage.getItem('format-selection') === 'true'
)

export const fileRenamingState = atomWithStorage(
  'file-renaming',
  localStorage.getItem('file-renaming') === 'true'
)

export const autoFileExtensionState = atomWithStorage(
  'auto-file-extension',
  localStorage.getItem('auto-file-extension') === 'true'
)

export const pathOverridingState = atomWithStorage(
  'path-overriding',
  localStorage.getItem('path-overriding') === 'true'
)

export const enableCustomArgsState = atomWithStorage(
  'enable-custom-args',
  localStorage.getItem('enable-custom-args') === 'true'
)

export const servedFromReverseProxyState = atomWithStorage(
  'reverseProxy',
  localStorage.getItem('reverseProxy') === 'true' || window.location.port == ''
)

export const servedFromReverseProxySubDirState = atomWithStorage<string>(
  'reverseProxySubDir',
  localStorage.getItem('reverseProxySubDir') ?? ''
)

export const serverAddressAndPortState = atom((get) => {
  if (get(servedFromReverseProxySubDirState)) {
    return `${get(serverAddressState)}/${get(servedFromReverseProxySubDirState)}/`
      .replaceAll('"', '')    // XXX: atomWithStorage uses JSON.stringify to serialize
      .replaceAll('//', '/')  // which puts extra double quotes.
  }
  if (get(servedFromReverseProxyState)) {
    return `${get(serverAddressState)}`
      .replaceAll('"', '')
  }

  const sap = `${get(serverAddressState)}:${get(serverPortState)}`
    .replaceAll('"', '')

  return sap.endsWith('/') ? sap.slice(0, -1) : sap
})

export const serverURL = atom((get) =>
  `${window.location.protocol}//${get(serverAddressAndPortState)}`
)

export const rpcWebSocketEndpoint = atom((get) => {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const sap = get(serverAddressAndPortState)

  return `${proto}//${sap.endsWith('/') ? sap.slice(0, -1) : sap}/rpc/ws`
})

export const rpcHTTPEndpoint = atom((get) => {
  const proto = window.location.protocol
  const sap = get(serverAddressAndPortState)

  return `${proto}//${sap.endsWith('/') ? sap.slice(0, -1) : sap}/rpc/http`
})

// Cookie status reported by the server. The cookie file is configured by path
// (paths.cookie_path) and converted automatically, so the UI only reads state.
export type CookieStatus = {
  exists: boolean
  sourcePath?: string
  format?: string
  count: number
  httpOnly?: number
  session?: number
  expired?: number
  missing?: string[]
  modifiedAt?: string
  // Which cookie the downloads are using: "imported" or "automatic". The
  // newer of the two wins, and the server decides — so this is the one answer
  // to "which file is in effect".
  active?: string
  // Where that export is, and when it was written.
  activePath?: string
  activeModifiedAt?: string
  // The state of each site's session cookie.
  //
  // Only the session cookie is reported: a cookie file holds hundreds per
  // site and almost all are disposable, so a count says nothing about whether
  // the site works. The one cookie that is the session does.
  sites?: {
    site: string
    // Whether the site is one the interface has an icon for; when it is not,
    // `site` is the domain itself and gets the globe.
    known: boolean
    domains: string[]
    // The name of the session cookie, empty when it is not known for the site.
    cookie?: string
    // Whether the file has it: false means the export is signed out there.
    present: boolean
    // When it dies, empty for a session cookie.
    expiry?: string
    // Whether that date has passed.
    expired: boolean
  }[]
  // The export the user uploaded, when there is one.
  imported?: {
    path: string
    format: string
    count: number
    modifiedAt: string
    failures: number
  }
}

// A counter that makes the cookie status be fetched again.
//
// The status is an async atom with no inputs of its own, so it is computed
// once and remembered for the life of the page. That is wrong for this one:
// importing a file changes what the status says, and the moment it changes is
// the moment a person is looking at it. The dialog showed what was there
// before the import, which reads as the import having done nothing.
// The catalogue of video sites, from the server.
//
// It is the only list of them. The footer and the cookie dialog each had one
// of their own, and they disagreed: Facebook had an icon in the footer and was
// missing from the dialog that is about cookies. A site added on the server
// appears in both now, and one whose mark does not exist yet is drawn as a
// globe.
export type SiteCatalogueEntry = {
  key: string
  name: string
  hosts: string[]
  keyCookie?: string
}

export const siteCatalogueState = atom<Promise<SiteCatalogueEntry[]>>(async (get) => {
  const body = await pipe(
    ffetch<{ sites?: SiteCatalogueEntry[] }>(`${get(serverURL)}/api/v1/sites`),
    matchW(
      () => ({ sites: [] as SiteCatalogueEntry[] }),
      (r) => r,
    ),
  )()

  return body?.sites ?? []
})

export const cookieStatusRefreshState = atom(0)

export const cookieStatusState = atom<Promise<CookieStatus>>(async (get) => {
  // Read, so that bumping the counter makes this be computed again.
  get(cookieStatusRefreshState)

  return await pipe(
    ffetch<CookieStatus>(`${get(serverURL)}/api/v1/cookies/status`),
    matchW(
      () => ({ exists: false, count: 0 }) as CookieStatus,
      (r) => r
    )
  )()
})

const themeSelector = atom<ThemeNarrowed>((get) => {
  const theme = get(themeState)
  if ((theme === 'system' && prefersDarkMode()) || theme === 'dark') {
    return 'dark'
  }
  return 'light'
})

export const accentState = atomWithStorage<Accent>(
  'accent-color',
  localStorage.getItem('accent-color') as Accent ?? 'default',
)

export const settingsState = atom<SettingsState>((get) => ({
  serverAddr: get(serverAddressState),
  serverPort: get(serverPortState),
  language: get(languageState),
  theme: get(themeSelector),
  accent: get(accentState),
  cliArgs: get(latestCliArgumentsState),
  formatSelection: get(formatSelectionState),
  fileRenaming: get(fileRenamingState),
  autoFileExtension: get(autoFileExtensionState),
  pathOverriding: get(pathOverridingState),
  enableCustomArgs: get(enableCustomArgsState),
  servedFromReverseProxy: get(servedFromReverseProxyState),
}))
