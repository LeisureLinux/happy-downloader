import UploadFileIcon from '@mui/icons-material/UploadFile'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Tooltip,
  Typography
} from '@mui/material'
import { Suspense, useCallback, useEffect, useState } from 'react'
import { useAtomValue } from 'jotai'
import { useTheme } from '@mui/material/styles'
import { cookieStatusState, serverURL } from '../atoms/settings'
import { formatTimestamp } from '../utils'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'
import { isDark, siteIcon } from './SiteIcon'

// The cookie dialog.
//
// It is reached from the footer's cookie indicator, because that is where a
// person goes when a download fails with a bot wall: the indicator shows when
// the export was last replaced, which is the first thing to check. The import
// used to be three clicks deeper, in settings.
//
// The sites the application can download from are listed at the top, with
// their icons. A cookie is not only for YouTube — Douyin and Weibo refuse a
// request without the browser's own cookies — and a person looking at this
// dialog is usually here because one particular site is not working.

type Props = {
  open: boolean
  onClose: () => void
}

// The cookie itself, as the icon.
//
// Drawn here rather than taken from the icon set because the set has no
// cookie. It is defined before the component that uses it: a const below
// would be a temporal dead zone error, and the whole page would go down.
const CookieIconForDialog = () => (
  <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
    <path d="M12 2a10 10 0 1 0 10 10 4 4 0 0 1-4-4 4 4 0 0 1-4-4 4 4 0 0 1-2-2Zm-3.5 7a1.5 1.5 0 1 0 1.5 1.5A1.5 1.5 0 0 0 8.5 9Zm3 4a1.5 1.5 0 1 0 1.5 1.5A1.5 1.5 0 0 0 11.5 13Zm4-3a1.5 1.5 0 1 0 1.5 1.5A1.5 1.5 0 0 0 15.5 10Z" />
  </svg>
)

// The sites the application can download from, in the order they are most
// likely to be used.
const supportedSites = [
  'youtube',
  'bilibili',
  'douyin',
  'tiktok',
  'xiaohongshu',
  'weibo',
  'x',
  'instagram',
  'reddit',
  'vimeo',
  'twitch',
  'niconico',
]

// The cookie status is a promise — the atom fetches it — so it can only be
// read inside a Suspense boundary.
//
// Reading it without one is the "async Client Component" error, and it takes
// the whole page down: the component is not a component until the promise
// settles, and React does not know how to render that.
//
// The dialog is therefore split in two: the outer one reads the atom inside a
// Suspense, and the inner one does the work.
const CookieDialog: React.FC<Props> = ({ open, onClose }) => (
  <Suspense fallback={null}>
    <CookieDialogContent open={open} onClose={onClose} />
  </Suspense>
)

const CookieDialogContent: React.FC<Props> = ({ open, onClose }) => {
  const status = useAtomValue(cookieStatusState)
  const serverAddr = useAtomValue(serverURL)
  const theme = useTheme()
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) setError('')
  }, [open])

  const upload = useCallback(async (file: File) => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/import`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/octet-stream' },
        body: file,
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const result = await res.json()

      pushMessage(
        i18n.t('cookieImported').replace('{count}', String(result.count ?? 0)),
        'success'
      )

      onClose()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }, [serverAddr, i18n, onClose, pushMessage])

  const remove = useCallback(async () => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/import`, {
        method: 'DELETE',
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      pushMessage(i18n.t('cookieImportRemoved'), 'info')
      onClose()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }, [serverAddr, i18n, onClose, pushMessage])

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Stack direction="row" spacing={1} alignItems="center">
          <CookieIconForDialog />
          <span>{i18n.t('cookieDialogTitle')}</span>
        </Stack>
      </DialogTitle>

      <DialogContent>
        {/* The sites the application can download from.
            A cookie is not only for YouTube: Douyin and Weibo refuse a
            request without the browser's own cookies, and the browser is
            the only place those exist. */}
        <Box sx={{ mb: 2 }}>
          <Typography variant="caption" color="text.secondary">
            {i18n.t('supportedSites')}
          </Typography>
          <Stack direction="row" spacing={0.75} sx={{ mt: 0.5, flexWrap: 'wrap' }}>
            {supportedSites.map(site => (
              <Tooltip key={site} title={site}>
                <Box
                  component="span"
                  sx={{ display: 'inline-flex', alignItems: 'center' }}
                >
                  {siteIcon(site, theme.palette.primary.main, isDark(theme))}
                </Box>
              </Tooltip>
            ))}
          </Stack>
        </Box>

        {/* What is in use now. */}
        <Alert severity="info" sx={{ mb: 2 }}>
          <Typography variant="body2">
            {status.active === 'imported'
              ? i18n.t('cookieInUseImported')
              : i18n.t('cookieInUseAutomatic')}
          </Typography>
          {status.activePath && (
            <Typography
              variant="caption"
              sx={{ display: 'block', wordBreak: 'break-all', mt: 0.5 }}
            >
              {status.activePath}
            </Typography>
          )}
          {status.activeModifiedAt && (
            <Typography variant="caption" sx={{ display: 'block' }}>
              {i18n.t('cookieModifiedAt')} · {formatTimestamp(status.activeModifiedAt)}
            </Typography>
          )}
        </Alert>

        {/* Per site, because the file is one list for every site and the
            question is which site's session is stale. A single figure for
            the whole file answers the wrong question: a file can be
            enormous and have nothing for the site that is failing. */}
        {status.sites && status.sites.length > 0 && (
          <Box sx={{ mb: 2 }}>
            <Typography variant="caption" color="text.secondary">
              {i18n.t('cookieSitesTitle')}
            </Typography>

            <Stack spacing={0.5} sx={{ mt: 0.5 }}>
              {status.sites.slice(0, 12).map(s => {
                // Compared as a number, not as a truthiness test: `0 && ..`
                // is 0, and React renders the 0.
                const expiring = (s.expired ?? 0) > 0
                const noLogin = (s.missingLogin?.length ?? 0) > 0

                return (
                  <Stack
                    key={s.site}
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    sx={{ minWidth: 0 }}
                  >
                    <Box
                      component="span"
                      sx={{ display: 'inline-flex', width: 20, flexShrink: 0 }}
                    >
                      {siteIcon(
                        s.known ? s.site : undefined,
                        theme.palette.primary.main,
                        isDark(theme)
                      )}
                    </Box>

                    <Tooltip title={s.domains.join(' · ')}>
                      <Typography
                        variant="body2"
                        noWrap
                        sx={{ flexGrow: 1, minWidth: 0 }}
                      >
                        {s.site}
                      </Typography>
                    </Tooltip>

                    <Typography variant="caption" color="text.secondary">
                      {s.count}
                    </Typography>

                    {/* When this site's newest cookie dies. A session
                        cookie has no date; that is not an error.

                        Some of them having expired does not make the date
                        useless: the others are still good, and replacing the
                        date with "expired" hid the ones that are. The count
                        is shown beside it instead. */}
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ minWidth: 120, textAlign: 'right' }}
                    >
                      {s.newestExpiry
                        ? formatTimestamp(s.newestExpiry)
                        : i18n.t('cookieSiteSession')}
                    </Typography>

                    {expiring && (
                      <Typography
                        variant="caption"
                        color="error.main"
                        sx={{ flexShrink: 0 }}
                      >
                        {s.expired} {i18n.t('cookieSiteExpired')}
                      </Typography>
                    )}

                    {noLogin && (
                      <Tooltip title={`${i18n.t('cookieSiteGuest')}: ${s.missingLogin?.join(', ')}`}>
                        <WarningAmberIcon
                          fontSize="small"
                          sx={{ color: 'warning.main', flexShrink: 0 }}
                        />
                      </Tooltip>
                    )}
                  </Stack>
                )
              })}
            </Stack>
          </Box>
        )}

        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          {i18n.t('cookieImportHint')}
        </Typography>

        <Stack direction="row" spacing={1}>
          <Button
            variant="outlined"
            startIcon={<UploadFileIcon />}
            disabled={busy}
            component="label"
          >
            {i18n.t('cookieImportButton')}
            <input
              type="file"
              accept=".json,.txt,application/json,text/plain"
              hidden
              onChange={e => {
                const file = e.target.files?.[0]
                // Reset, so choosing the same file twice fires the change
                // event again.
                e.target.value = ''
                if (file) void upload(file)
              }}
            />
          </Button>

          {status.imported && (
            <Button color="error" disabled={busy} onClick={remove}>
              {i18n.t('cookieImportRemove')}
            </Button>
          )}
        </Stack>

        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            <Box component="span">{error}</Box>
          </Alert>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>{i18n.t('close')}</Button>
      </DialogActions>
    </Dialog>
  )
}

export default CookieDialog
