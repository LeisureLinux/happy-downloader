import { ThemeProvider } from '@emotion/react'
import ChevronLeft from '@mui/icons-material/ChevronLeft'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import Dashboard from '@mui/icons-material/Dashboard'
import Menu from '@mui/icons-material/Menu'
import SettingsIcon from '@mui/icons-material/Settings'
import { Box, createTheme } from '@mui/material'
import CssBaseline from '@mui/material/CssBaseline'
import Divider from '@mui/material/Divider'
import IconButton from '@mui/material/IconButton'
import List from '@mui/material/List'
import ListItemButton from '@mui/material/ListItemButton'
import ListItemIcon from '@mui/material/ListItemIcon'
import ListItemText from '@mui/material/ListItemText'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import { grey } from '@mui/material/colors'
import { useAtom, useAtomValue } from 'jotai'
import { Suspense, useMemo, useState } from 'react'
import { Link, Outlet } from 'react-router-dom'
import { settingsState } from './atoms/settings'
import { logPanelOpenAtom } from './atoms/ui'
import AppBar from './components/AppBar'
import AppVersion from './components/AppVersion'
import Drawer from './components/Drawer'
import Footer from './components/Footer'
import LogPanel from './components/LogPanel'
import SocketSubscriber from './components/SocketSubscriber'
import { useI18n } from './hooks/useI18n'
import Toaster from './providers/ToasterProvider'
import { getAccentValue } from './utils'

// PRODUCT_NAME is the app name. It used to be a user setting, but renaming the
// product is not something a user does, and the field only produced a stale
// localStorage value that overrode the real name.
const PRODUCT_NAME = 'Happy Downloader'

export default function Layout() {
  const [open, setOpen] = useState(false)

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

  const toggleDrawer = () => setOpen(state => !state)

  const { i18n } = useI18n()

  return (
    <ThemeProvider theme={theme}>
      <title>{PRODUCT_NAME}</title>
      <SocketSubscriber />
      <Box sx={{ display: 'flex' }}>
        <CssBaseline />
        <AppBar position="absolute" open={open}>
          <Toolbar sx={{ pr: '24px' }}>
            <IconButton
              edge="start"
              color="inherit"
              aria-label="open drawer"
              onClick={toggleDrawer}
              sx={{
                marginRight: '36px',
                ...(open && { display: 'none' }),
              }}
            >
              <Menu />
            </IconButton>
            <Typography
              component="h1"
              variant="h6"
              color="inherit"
              noWrap
              sx={{ flexGrow: 1 }}
            >
              {PRODUCT_NAME}
              <AppVersion />
            </Typography>
          </Toolbar>
        </AppBar>
        <Drawer variant="permanent" open={open}>
          <Toolbar
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'flex-end',
              px: [1],
            }}
          >
            <IconButton onClick={toggleDrawer}>
              <ChevronLeft />
            </IconButton>
          </Toolbar>
          <Divider />
          <List component="nav">
            <Link to={'/'} style={
              {
                textDecoration: 'none',
                color: mode === 'dark' ? '#ffffff' : '#000000DE'
              }
            }>
              <ListItemButton>
                <ListItemIcon>
                  <CloudDownloadIcon />
                </ListItemIcon>
                <ListItemText primary={i18n.t('downloadsButtonLabel')} />
              </ListItemButton>
            </Link>
            
            <Link to={'/filebrowser'} style={
              {
                textDecoration: 'none',
                color: mode === 'dark' ? '#ffffff' : '#000000DE'
              }
            }>
              <ListItemButton>
                <ListItemIcon>
                  <Dashboard />
                </ListItemIcon>
                <ListItemText primary={i18n.t('browseButtonLabel')} />
              </ListItemButton>
            </Link>
            <Link to={'/settings'} style={
              {
                textDecoration: 'none',
                color: mode === 'dark' ? '#ffffff' : '#000000DE'
              }
            }>
              <ListItemButton>
                <ListItemIcon>
                  <SettingsIcon />
                </ListItemIcon>
                <ListItemText primary={i18n.t('settingsButtonLabel')} />
              </ListItemButton>
            </Link>
          </List>
        </Drawer>
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