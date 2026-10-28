import { ThemeProvider } from '@emotion/react'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import Dashboard from '@mui/icons-material/Dashboard'
import SettingsIcon from '@mui/icons-material/Settings'
import { Box, Button, createTheme, Tooltip } from '@mui/material'
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
  // The log button lives in the footer, not here: on a phone the header had
  // four labelled buttons next to the product name, and they were squeezed
  // into an unreadable row. The log is a thing you consult while watching
  // something else, which is what the footer is for.
  const toggles = [
    {
      key: 'settings',
      label: i18n.t('settingsButtonLabel'),
      icon: <SettingsIcon fontSize="small" />,
      open: settingsOpen,
      // Both panels occupy the same corner, so opening one closes the other.
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
          {/* The header has to work on a phone.
              Four labelled buttons beside the product name did not: the
              title took what it wanted and the buttons were squeezed into
              an unreadable row. So the title gives up its width first, the
              buttons keep their labels as long as there is room for them,
              and below a phone's width they become icons — which is what
              they are anyway, with a tooltip for the name. */}
          <Toolbar sx={{ pr: { xs: 1, sm: 2 }, gap: { xs: 0.25, sm: 1 }, flexWrap: 'nowrap' }}>
            <Typography
              component="h1"
              variant="h6"
              noWrap
              sx={{
                flexGrow: 1,
                minWidth: 0,
                color: 'primary.main',
                fontSize: { xs: '1rem', sm: '1.25rem' },
              }}
            >
              {productName}
              {/* The version takes space from the buttons on a phone, and
                  it is also the first thing to go: the header is for the
                  product name and the controls. */}
              <Box sx={{ display: { xs: 'none', sm: 'inline' } }}>
                <AppVersion />
              </Box>
            </Typography>

            {/* 顺序：设置 | 浏览 | 下载。
                前一个是开关，后两个是目的地。

                color="inherit" would render these in the app bar's own
                contrast colour, which is white in dark mode — the accent
                would never reach them. They use the accent explicitly, dimmed
                when inactive, so the chosen colour shows on the controls too. */}
            {toggles.map(t => (
              <Tooltip key={t.key} title={t.label}>
                <Button
                  size="small"
                  onClick={t.onToggle}
                  aria-pressed={t.open}
                  sx={{
                    color: 'primary.main',
                    opacity: t.open ? 1 : 0.72,
                    fontWeight: t.open ? 600 : 400,
                    '&:hover': { opacity: 1 },
                    // The label is what does not fit on a phone; the icon is
                    // the control, and the tooltip names it.
                    minWidth: { xs: 34, sm: 'auto' },
                    px: { xs: 0.5, sm: 1 },
                  }}
                >
                  {t.icon}
                  <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' }, ml: 0.5 }}>
                    {t.label}
                  </Box>
                </Button>
              </Tooltip>
            ))}

            {destinations.map(d => (
              <Tooltip key={d.to} title={d.label}>
                <Button
                  component={Link}
                  to={d.to}
                  size="small"
                  aria-current={pathname === d.to ? 'page' : undefined}
                  sx={{
                    color: 'primary.main',
                    opacity: pathname === d.to ? 1 : 0.72,
                    fontWeight: pathname === d.to ? 600 : 400,
                    '&:hover': { opacity: 1 },
                    minWidth: { xs: 34, sm: 'auto' },
                    px: { xs: 0.5, sm: 1 },
                  }}
                >
                  {d.icon}
                  <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' }, ml: 0.5 }}>
                    {d.label}
                  </Box>
                </Button>
              </Tooltip>
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