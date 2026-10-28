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
import { logPanelOpenAtom } from './atoms/ui'
import AppBar from './components/AppBar'
import AppVersion from './components/AppVersion'
import Footer from './components/Footer'
import LogPanel from './components/LogPanel'
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
  // The log is a toggle rather than a destination: reading the log is done
  // while watching a download, and a page would hide the download list. It
  // lives in the footer.
  // The log button lives in the footer, not here: on a phone the header had
  // four labelled buttons next to the product name, and they were squeezed
  // into an unreadable row. The log is a thing you consult while watching
  // something else, which is what the footer is for.
  // 顺序：设置 | 浏览。
  //
  // 设置放最左边：它是两个里面最不常点的一个，放在最左边就不用每次
  // 都越过浏览才能碰到它。
  //
  // 下载不在这里：它是首页，按钮在页面里，不必在顶上再放一个。顶栏
  // 上的下载链接在大标题旁边，点标题和点那个图标是一回事。
  const destinations = [
    { to: '/settings', label: i18n.t('settingsButtonLabel'), icon: <SettingsIcon fontSize="small" /> },
    { to: '/filebrowser', label: i18n.t('browseButtonLabel'), icon: <Dashboard fontSize="small" /> },
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
            {/* 产品名本身就是下载页的链接，旁边的图标也是。
                顶上再放一个"下载"按钮是重复的，而重复的那个被点到的
                次数最多 —— 所以它留在了大标题上。 */}
            <Box
              component={Link}
              to="/"
              aria-label={i18n.t('downloadsButtonLabel')}
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
                minWidth: 0,
                color: 'primary.main',
                textDecoration: 'none',
                '&:hover': { opacity: 0.72 },
              }}
            >
              <Typography
                component="h1"
                variant="h6"
                noWrap
                sx={{
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

              <CloudDownloadIcon fontSize="small" />
            </Box>

            <Box sx={{ flexGrow: 1 }} />

            {/* 顺序：设置 | 浏览 | 下载。
                前一个是开关，后两个是目的地。

                color="inherit" would render these in the app bar's own
                contrast colour, which is white in dark mode — the accent
                would never reach them. They use the accent explicitly, dimmed
                when inactive, so the chosen colour shows on the controls too. */}
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
      <Footer />
      <Toaster />
    </ThemeProvider>
  )
}