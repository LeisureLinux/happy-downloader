import { ThemeProvider } from '@emotion/react'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import Dashboard from '@mui/icons-material/Dashboard'
import SettingsIcon from '@mui/icons-material/Settings'
import TerminalIcon from '@mui/icons-material/Terminal'
import { Box, Button, createTheme } from '@mui/material'
import CssBaseline from '@mui/material/CssBaseline'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import { grey } from '@mui/material/colors'
import { useAtom, useAtomValue } from 'jotai'
import { useMemo } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import { settingsState } from './atoms/settings'
import { logPanelOpenAtom, settingsPanelOpenAtom } from './atoms/ui'
import AppBar from './components/AppBar'
import AppVersion from './components/AppVersion'
import Footer from './components/Footer'
import LogPanel from './components/LogPanel'
import SettingsPanel from './components/SettingsPanel'
import CaptionJobWatcher from './components/CaptionJobWatcher'
import SocketSubscriber from './components/SocketSubscriber'
import { useI18n } from './hooks/useI18n'
import Toaster from './providers/ToasterProvider'
import { getAccentValue } from './utils'

// The app name is translated rather than hardcoded: it is the most-read string
// on the page, and a Chinese user should see it in Chinese.

export default function Layout() {
  const settings = useAtomValue(settingsState)
  const [logOpen, setLogOpen] = useAtom(logPanelOpenAtom)
  const [settingsOpen, setSettingsOpen] = useAtom(settingsPanelOpenAtom)

  const mode = settings.theme
  const theme = useMemo(() =>
    createTheme({
      palette: {
        mode: settings.theme,
        primary: {
          main: getAccentValue(settings.accent, settings.theme)
        },
        background: {
          default: settings.theme === 'light' ? grey[50] : '#121212'
        },
      },
    }), [settings.theme, settings.accent]
  )

  const { i18n } = useI18n()
  const { pathname } = useLocation()

  const productName = i18n.t('appName')

  // Two panels and two destinations. The drawer that used to hold them was a
  // permanent 240px column for three links, and it cost the page that width on
  // every screen.
  //
  // Logs and settings are toggles rather than destinations: both are things you
  // consult while watching the page underneath, so they overlay it instead of
  // replacing it. Only Browse and Downloads are places you go.
  const toggles = [
    {
      key: 'log',
      label: i18n.t('logsTitle'),
      icon: <TerminalIcon fontSize="small" />,
      open: logOpen,
      // Both panels occupy the same corner, so opening one closes the other.
      onToggle: () => { setLogOpen(v => !v); setSettingsOpen(false) },
    },
    {
      key: 'settings',
      label: i18n.t('settingsButtonLabel'),
      icon: <SettingsIcon fontSize="small" />,
      open: settingsOpen,
      onToggle: () => { setSettingsOpen(v => !v); setLogOpen(false) },
    },
  ]

  const destinations = [
    { to: '/filebrowser', label: i18n.t('browseButtonLabel'), icon: <Dashboard fontSize="small" /> },
    { to: '/', label: i18n.t('downloadsButtonLabel'), icon: <CloudDownloadIcon fontSize="small" /> },
  ]

  return (
    <ThemeProvider theme={theme}>
      <title>{productName}</title>
      <SocketSubscriber />
      {/* The caption downloads are watched here, not in the dialog that starts
          them: the dialog is gone by the time the work finishes, and the file
          browser has to be able to say what is happening to the row. */}
      <CaptionJobWatcher />
      <Box sx={{ display: 'flex' }}>
        <CssBaseline />
        <AppBar position="absolute">
          <Toolbar sx={{ pr: 2, gap: 1 }}>
            <Typography
              component="h1"
              variant="h6"
              noWrap
              sx={{ flexGrow: 1, color: 'primary.main' }}
            >
              {productName}
              <AppVersion />
            </Typography>

            {/* 顺序：日志 | 设置 | 浏览 | 下载。
                前两个是开关，后两个是目的地。

                color="inherit" would render these in the app bar's own
                contrast colour, which is white in dark mode — the accent
                would never reach them. They use the accent explicitly, dimmed
                when inactive, so the chosen colour shows on the controls too. */}
            {toggles.map(t => (
              <Button
                key={t.key}
                size="small"
                onClick={t.onToggle}
                startIcon={t.icon}
                aria-pressed={t.open}
                sx={{
                  color: 'primary.main',
                  opacity: t.open ? 1 : 0.72,
                  fontWeight: t.open ? 600 : 400,
                  '&:hover': { opacity: 1 },
                }}
              >
                {t.label}
              </Button>
            ))}

            {destinations.map(d => (
              <Button
                key={d.to}
                component={Link}
                to={d.to}
                size="small"
                startIcon={d.icon}
                sx={{
                  color: 'primary.main',
                  opacity: pathname === d.to ? 1 : 0.72,
                  fontWeight: pathname === d.to ? 600 : 400,
                  '&:hover': { opacity: 1 },
                }}
              >
                {d.label}
              </Button>
            ))}
          </Toolbar>
        </AppBar>
        <Box
          component="main"
          sx={{
            flexGrow: 1,
            height: '100vh',
            overflow: 'auto',
          }}
        >
          <Toolbar />
          <Outlet />
        </Box>
      </Box>


      <LogPanel open={logOpen} onClose={() => setLogOpen(false)} />
      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <Footer />
      <Toaster />
    </ThemeProvider>
  )
}