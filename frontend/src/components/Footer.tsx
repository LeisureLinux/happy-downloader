import DownloadIcon from '@mui/icons-material/Download'
import GitHubIcon from '@mui/icons-material/GitHub'
import SettingsEthernet from '@mui/icons-material/SettingsEthernet'
import TerminalIcon from '@mui/icons-material/Terminal'
import { AppBar, Box, Divider, IconButton, Toolbar, Tooltip } from '@mui/material'
import { Suspense } from 'react'
import { settingsState } from '../atoms/settings'
import { connectedState } from '../atoms/status'
import { totalDownloadSpeedState } from '../atoms/ui'
import { useI18n } from '../hooks/useI18n'
import { formatSpeedMiB } from '../utils'
import CookieIndicator from './CookieIndicator'
import FreeSpaceIndicator from './FreeSpaceIndicator'
import YtdlpIndicator from './YtdlpIndicator'
import { useAtom, useAtomValue } from 'jotai'
import { logPanelOpenAtom, settingsPanelOpenAtom } from '../atoms/ui'

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
const Footer: React.FC = () => {
  const settings = useAtomValue(settingsState)
  const isConnected = useAtomValue(connectedState)
  const totalDownloadSpeed = useAtomValue(totalDownloadSpeedState)

  const [logOpen, setLogOpen] = useAtom(logPanelOpenAtom)
  const [, setSettingsOpen] = useAtom(settingsPanelOpenAtom)

  const mode = settings.theme
  const { i18n } = useI18n()

  const toggleLog = () => {
    setLogOpen(v => !v)
    // Both panels occupy the same corner, so opening one closes the other.
    setSettingsOpen(false)
  }

  return (
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

            {/* The host address is the first thing to go on a narrow screen:
                it is the least useful of the four, and the connection state
                is the part that matters. */}
            <Box
              sx={{
                display: { xs: 'none', md: 'flex' },
                alignItems: 'center',
                gap: 0.5,
                flexShrink: 0,
              }}
            >
              <Divider orientation="vertical" flexItem />
              <SettingsEthernet sx={{ color: 'primary.main' }} />
              <Box component="span" sx={{ color: 'primary.main' }}>
                {isConnected ? settings.serverAddr : i18n.t('notConnectedText')}
              </Box>
            </Box>

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
                <CookieIndicator />
              </Suspense>
            </Box>
          </Box>
        </Box>

        {/* The repository, at the far right. */}
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
  )
}

export default Footer
