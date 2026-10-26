import { ThemeProvider } from '@emotion/react'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import Dashboard from '@mui/icons-material/Dashboard'
import SettingsIcon from '@mui/icons-material/Settings'
import { Box, Button, Fab, Tooltip, createTheme } from '@mui/material'
import CssBaseline from '@mui/material/CssBaseline'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import { grey } from '@mui/material/colors'
import { useAtom, useAtomValue } from 'jotai'
import { useMemo } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { settingsState } from './atoms/settings'
import { logPanelOpenAtom } from './atoms/ui'
import AppBar from './components/AppBar'
import AppVersion from './components/AppVersion'
import Footer from './components/Footer'
import LogPanel, { LogToggle } from './components/LogPanel'
import SocketSubscriber from './components/SocketSubscriber'
import { useI18n } from './hooks/useI18n'
import Toaster from './providers/ToasterProvider'
import { getAccentValue } from './utils'

// The app name is translated rather than hardcoded: it is the most-read string
// on the page, and a Chinese user should see it in Chinese.

export default function Layout() {
  const settings = useAtomValue(settingsState)
  const [logOpen, setLogOpen] = useAtom(logPanelOpenAtom)

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
  const navigate = useNavigate()
  const { pathname } = useLocation()

  const productName = i18n.t('appName')

  // Two destinations and one setting. The drawer that used to hold them was a
  // permanent 240px column for three links, and it cost the page that width on
  // every screen.
  const destinations = [
    { to: '/', label: i18n.t('downloadsButtonLabel'), icon: <CloudDownloadIcon fontSize="small" /> },
    { to: '/filebrowser', label: i18n.t('browseButtonLabel'), icon: <Dashboard fontSize="small" /> },
  ]

  return (
    <ThemeProvider theme={theme}>
      <title>{productName}</title>
      <SocketSubscriber />
      <Box sx={{ display: 'flex' }}>
        <CssBaseline />
        <AppBar position="absolute">
          <Toolbar sx={{ pr: 2, gap: 1 }}>
            <Typography component="h1" variant="h6" color="inherit" noWrap sx={{ flexGrow: 1 }}>
              {productName}
              <AppVersion />
            </Typography>

            {/* 下载 / 浏览 放到右上角：两个目的地，够用不上侧边栏。 */}
            {destinations.map(d => (
              <Button
                key={d.to}
                component={Link}
                to={d.to}
                color="inherit"
                size="small"
                startIcon={d.icon}
                sx={{
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

      {/* 设置放左下角，和右下角的日志按钮对称；两者都抬到底部状态条之上，
          否则会压在速度和剩余空间上。 */}
      <Tooltip title={i18n.t('settingsButtonLabel')} placement="right">
        <Fab
          size="small"
          color={pathname === '/settings' ? 'primary' : 'default'}
          onClick={() => navigate('/settings')}
          sx={{ position: 'fixed', left: 16, bottom: 64, zIndex: 1400 }}
        >
          <SettingsIcon fontSize="small" />
        </Fab>
      </Tooltip>

      <LogPanel open={logOpen} onClose={() => setLogOpen(false)} />
      <LogToggle open={logOpen} onToggle={() => setLogOpen(v => !v)} />
      <Footer />
      <Toaster />
    </ThemeProvider>
  )
}