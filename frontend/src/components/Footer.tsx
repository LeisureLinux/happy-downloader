import DownloadIcon from '@mui/icons-material/Download'
import GitHubIcon from '@mui/icons-material/GitHub'
import TerminalIcon from '@mui/icons-material/Terminal'
import { AppBar, Box, Divider, IconButton, Stack, Toolbar, Tooltip } from '@mui/material'
import { Suspense, useState } from 'react'
import { settingsState, siteCatalogueState } from '../atoms/settings'
import { connectedState } from '../atoms/status'
import { totalDownloadSpeedState } from '../atoms/ui'
import { useI18n } from '../hooks/useI18n'
import { formatSpeedMiB } from '../utils'
import CookieDialog from './CookieDialog'
import { isDark, siteIcon } from './SiteIcon'
import CookieIndicator from './CookieIndicator'
import FreeSpaceIndicator from './FreeSpaceIndicator'
import YtdlpIndicator from './YtdlpIndicator'
import { useAtom, useAtomValue } from 'jotai'
import { useTheme } from '@mui/material/styles'
import { logPanelOpenAtom } from '../atoms/ui'

// The bottom bar.
//
// Two groups: the log button at the far left, and the repository link at the
// far right, with the status in between.
//
// The log is here because it is a thing you consult while watching a
// download, and the header had four labelled buttons squeezed next to the
// product name on a phone. The repository link is at the other end because it
// is not navigation at all — it is where this deployment came from — and the
// end of the bar is where something like that belongs.
// The sites, as icons.
//
// Split out because it is the only part of the footer that waits for the
// server: the catalogue is a promise, and a promise read outside a Suspense
// boundary is a crash.
const SiteIcons: React.FC = () => {
  const catalogue = useAtomValue(siteCatalogueState)
  const theme = useTheme()

  return (
    <>
      {catalogue.map(site => (
        <Tooltip key={site.key} title={site.name}>
          <Box
            component="span"
            sx={{ display: 'inline-flex', alignItems: 'center' }}
          >
            {siteIcon(site.key, theme.palette.primary.main, isDark(theme))}
          </Box>
        </Tooltip>
      ))}
    </>
  )
}

const Footer: React.FC = () => {
  const settings = useAtomValue(settingsState)
  const isConnected = useAtomValue(connectedState)
  const totalDownloadSpeed = useAtomValue(totalDownloadSpeedState)

  // The cookie dialog, opened from the cookie indicator in the bar.
  const [cookieDialog, setCookieDialog] = useState(false)

  const [logOpen, setLogOpen] = useAtom(logPanelOpenAtom)

  const mode = settings.theme
  const theme = useTheme()
  const { i18n } = useI18n()

  const toggleLog = () => setLogOpen(v => !v)

  return (
    <>
    <AppBar position="fixed" color="default" sx={{
      top: 'auto',
      bottom: 0,
      height: 48,
      zIndex: 1200,
      borderTop: mode === 'light'
        ? '1px solid rgba(0, 0, 0, 0.12)'
        : '1px solid rgba(255, 255, 255, 0.12)',
    }}>
      {/* paddingBottom made the toolbar 64px tall inside a 48px bar, so the
          row overflowed the viewport and the icons were clipped at the bottom
          edge. The bar centres its own content; nothing needs to be pushed.

          The row does not wrap: a footer that wraps is two rows tall on a
          phone, and the page content has to make room for it. The parts that
          are least important — the host address, the version — are the ones
          that give up their width first. */}
      <Toolbar sx={{
        minHeight: '48px !important',
        fontSize: 14,
        display: 'flex',
        gap: 1,
        justifyContent: 'space-between',
        px: { xs: 0.5, sm: 2 },
        overflow: 'hidden',
      }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: { xs: 0.5, sm: 1 }, minWidth: 0 }}>
          {/* The log, at the far left. */}
          <Tooltip title={i18n.t('logsTitle')}>
            <IconButton
              size="small"
              onClick={toggleLog}
              aria-pressed={logOpen}
              sx={{ color: 'primary.main', opacity: logOpen ? 1 : 0.72 }}
            >
              <TerminalIcon fontSize="small" />
            </IconButton>
          </Tooltip>

          <Divider orientation="vertical" flexItem />

          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: { xs: 1, sm: 2 },
              minWidth: 0,
              overflow: 'hidden',
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 }}>
              <DownloadIcon sx={{ color: 'primary.main' }} />
              <Box component="span" sx={{ color: 'primary.main' }}>
                {formatSpeedMiB(totalDownloadSpeed)}
              </Box>
            </Box>

            {/* The host address is not shown: it is the address the browser
                is already on, so it tells nobody anything. The connection
                state is worth knowing, and it is the only thing left here —
                a page that has lost the server should say so rather than
                quietly showing stale numbers. */}
            {!isConnected && (
              <Box sx={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
                <Divider orientation="vertical" flexItem />
                <Box component="span" sx={{ color: 'error.main', ml: 1 }}>
                  {i18n.t('notConnectedText')}
                </Box>
              </Box>
            )}

            <Box sx={{ display: { xs: 'none', sm: 'flex' }, alignItems: 'center', flexShrink: 0 }}>
              <Divider orientation="vertical" flexItem />
              <Suspense fallback={i18n.t('loadingLabel')}>
                <FreeSpaceIndicator />
              </Suspense>
            </Box>

            {/* The yt-dlp version, then the cookie export's timestamp. Both
                are answers to "can this download work right now": yt-dlp
                breaks YouTube every few weeks, and a stale cookie fails the
                same way. */}
            <Box sx={{ display: { xs: 'none', lg: 'flex' }, alignItems: 'center', flexShrink: 0 }}>
              <Divider orientation="vertical" flexItem />
              <Suspense fallback={null}>
                <YtdlpIndicator />
              </Suspense>
            </Box>

            <Box sx={{ display: { xs: 'none', md: 'flex' }, alignItems: 'center', flexShrink: 0 }}>
              <Divider orientation="vertical" flexItem />
              <Suspense fallback={null}>
                <CookieIndicator onImport={() => setCookieDialog(true)} />
              </Suspense>
            </Box>
          </Box>
        </Box>

        {/* The sites the application can download from, at the far
            right. They are what the icons in the file browser are keyed
            by, and a person looking at a file that will not play wants to
            know what else it could have come from. */}
        <Stack
            direction="row"
            spacing={0.5}
            alignItems="center"
            sx={{ flexShrink: 0, mr: 0.5 }}
          >
            {/* The catalogue arrives as a promise, so it is read inside a
                Suspense boundary like every other indicator here. Reading an
                async atom without one takes the page down with a minified
                React error that says nothing about what was wrong. */}
            <Suspense fallback={null}>
              <SiteIcons />
            </Suspense>
          </Stack>

          <Tooltip title={i18n.t('sourceCode')}>
            <IconButton
              size="small"
              component="a"
              href="https://github.com/LeisureLinux/happy-downloader"
              target="_blank"
              rel="noopener noreferrer"
              sx={{ color: 'primary.main', flexShrink: 0 }}
            >
              <GitHubIcon fontSize="small" />
            </IconButton>
          </Tooltip>
      </Toolbar>
    </AppBar>

    {/* The cookie dialog, opened by clicking the cookie indicator in the
        bar. It was three clicks deep in settings, which is a long way to
        go when a download has just failed with a bot wall. */}
    <CookieDialog
      open={cookieDialog}
      onClose={() => setCookieDialog(false)}
    />
    </>
  )
}

export default Footer
