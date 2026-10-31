import {
  Box,
  Checkbox,
  Container,
  FormControl,
  FormControlLabel,
  Grid,
  InputAdornment,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  SelectChangeEvent,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material'
import { useAtom } from 'jotai'
import { Suspense, useCallback, useEffect, useMemo, useState } from 'react'
import {
  Subject,
  debounceTime,
  distinctUntilChanged,
  map,
  takeWhile
} from 'rxjs'
import {
  Accent,
  Language,
  Theme,
  accentState,
  accents,
  enableCustomArgsState,
  fileRenamingState,
  autoFileExtensionState,
  formatSelectionState,
  languageState,
  languageNames,
  languages,
  pathOverridingState,
  servedFromReverseProxyState,
  servedFromReverseProxySubDirState,
  serverAddressState,
  serverPortState,
  themeState
} from '../atoms/settings'
import DownloadCredentials from '../components/DownloadCredentials'
import NetworkSettings from '../components/NetworkSettings'
import QuitButton from '../components/QuitButton'
import CookiesFromBrowser from '../components/CookiesFromBrowser'
import { useToast } from '../hooks/toast'
import { useI18n } from '../hooks/useI18n'
import Translator from '../lib/i18n'
import { validateDomain, validateIP } from '../utils'

// NEED ABSOLUTELY TO BE SPLIT IN MULTIPLE COMPONENTS
export function SettingsContent() {
  const [reverseProxy, setReverseProxy] = useAtom(servedFromReverseProxyState)
  const [baseURL, setBaseURL] = useAtom(servedFromReverseProxySubDirState)

  const [formatSelection, setFormatSelection] = useAtom(formatSelectionState)
  const [pathOverriding, setPathOverriding] = useAtom(pathOverridingState)
  const [fileRenaming, setFileRenaming] = useAtom(fileRenamingState)
  const [autoFileExtension, setAutoFileExtension] = useAtom(autoFileExtensionState)
  const [enableArgs, setEnableArgs] = useAtom(enableCustomArgsState)

  const [serverAddr, setServerAddr] = useAtom(serverAddressState)
  const [serverPort, setServerPort] = useAtom(serverPortState)

  const [language, setLanguage] = useAtom(languageState)
  const [accent, setAccent] = useAtom(accentState)

  const [theme, setTheme] = useAtom(themeState)

  const [invalidIP, setInvalidIP] = useState(false)

  const { i18n, tSub } = useI18n()

  const { pushMessage } = useToast()

  const baseURL$ = useMemo(() => new Subject<string>(), [])
  const serverAddr$ = useMemo(() => new Subject<string>(), [])
  const serverPort$ = useMemo(() => new Subject<string>(), [])


  useEffect(() => {
    const sub = baseURL$
      .pipe(debounceTime(500))
      .subscribe(baseURL => {
        setBaseURL(baseURL)
        pushMessage(i18n.t('restartAppMessage'), 'info')
      })
    return () => sub.unsubscribe()
  }, [])

  useEffect(() => {
    const sub = serverAddr$
      .pipe(
        debounceTime(500),
        distinctUntilChanged()
      )
      .subscribe(addr => {
        if (validateIP(addr)) {
          setInvalidIP(false)
          setServerAddr(addr)
          pushMessage(i18n.t('restartAppMessage'), 'info')
        } else if (validateDomain(addr)) {
          setInvalidIP(false)
          setServerAddr(addr)
          pushMessage(i18n.t('restartAppMessage'), 'info')
        } else {
          setInvalidIP(true)
        }
      })
    return () => sub.unsubscribe()
  }, [serverAddr$])

  useEffect(() => {
    const sub = serverPort$
      .pipe(
        debounceTime(500),
        map(val => Number(val)),
        takeWhile(val => isFinite(val) && val <= 65535),
      )
      .subscribe(port => {
        setServerPort(port)
        pushMessage(i18n.t('restartAppMessage'), 'info')
      })
    return () => sub.unsubscribe()
  }, [])

  /**
   * Language toggler handler 
   */
  const handleLanguageChange = (event: SelectChangeEvent<Language>) => {
    setLanguage(event.target.value as Language)

    // Synchronous now, so the re-render triggered by the atom above already
    // reads the new strings. The old 100 ms forceUpdate existed only to paper
    // over the dynamic import.
    Translator.instance.setLanguage(event.target.value)
  }

  /**
   * Theme toggler handler 
   */
  const handleThemeChange = (event: SelectChangeEvent<Theme>) => {
    setTheme(event.target.value as Theme)
  }

  return (
    <Box sx={{ p: 2.5 }}>
        <Grid container spacing={2}>
          <Grid item xs={12} md={9}>
            <TextField
              fullWidth
              label={i18n.t('serverAddressTitle')}
              defaultValue={serverAddr}
              error={invalidIP}
              onChange={(e) => serverAddr$.next(e.currentTarget.value)}
              InputProps={{
                startAdornment: <InputAdornment position="start">ws://</InputAdornment>,
              }}
            />
          </Grid>
          <Grid item xs={12} md={3}>
            <TextField
              disabled={reverseProxy}
              fullWidth
              label={i18n.t('serverPortTitle')}
              defaultValue={serverPort}
              onChange={(e) => serverPort$.next(e.currentTarget.value)}
              error={isNaN(Number(serverPort)) || Number(serverPort) > 65535}
            />
          </Grid>
        </Grid>
        <Typography variant="h6" color="primary" sx={{ mt: 0.5, mb: 2 }}>
          {i18n.t('appearanceAnchor')}
        </Typography>
        <Grid container spacing={2}>
          <Grid item xs={12} md={4}>
            <FormControl fullWidth>
              <InputLabel>{i18n.t('languageSelect')}</InputLabel>
              <Select
                defaultValue={language}
                label={i18n.t('languageSelect')}
                onChange={handleLanguageChange}
              >
                {[...languages].sort((a, b) => languageNames[a].localeCompare(languageNames[b])).map(l => (
                  <MenuItem value={l} key={l}>
                    {languageNames[l]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
          <Grid item xs={12} md={4}>
            <FormControl fullWidth>
              <InputLabel>{i18n.t('themeSelect')}</InputLabel>
              <Select
                defaultValue={theme}
                label={i18n.t('themeSelect')}
                onChange={handleThemeChange}
              >
                <MenuItem value="light">{i18n.t('lightThemeButton')}</MenuItem>
                <MenuItem value="dark">{i18n.t('darkThemeButton')}</MenuItem>
                <MenuItem value="system">{i18n.t('systemThemeOption')}</MenuItem>
              </Select>
            </FormControl>
          </Grid>
          <Grid item xs={12} md={4}>
            <FormControl fullWidth>
              <InputLabel>{i18n.t('accentSelect')}</InputLabel>
              <Select
                defaultValue={accent}
                label={i18n.t('accentSelect')}
                onChange={(e) => setAccent(e.target.value as Accent)}
              >
                {accents.map((accent) => (
                  <MenuItem key={accent} value={accent}>
                    {tSub('accentNames', accent)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
        </Grid>
        <Typography variant="h6" color="primary" sx={{ mt: 2, mb: 0.5 }}>
          {i18n.t('generalDownloadSettings')}
        </Typography>

        <FormControlLabel
          control={
            <Switch
              defaultChecked={formatSelection}
              onChange={() => {
                setFormatSelection(!formatSelection)
              }}
            />
          }
          label={i18n.t('formatSelectionEnabler')}
        />
        <Grid>
          <Typography variant="h6" color="primary" sx={{ mt: 2, mb: 0.5 }}>
            {i18n.t('overridesAnchor')}
          </Typography>
          <Stack direction="column">
            <FormControlLabel
              control={
                <Switch
                  defaultChecked={!!pathOverriding}
                  onChange={() => {
                    setPathOverriding(state => !state)
                  }}
                />
              }
              label={i18n.t('pathOverrideOption')}
            />
            <FormControlLabel
              control={
                <Switch
                  defaultChecked={fileRenaming}
                  onChange={() => {
                    if (fileRenaming) {
                      setAutoFileExtension(false)
                    }
                    setFileRenaming(state => !state)
                  }}
                />
              }
              label={i18n.t('filenameOverrideOption')}
            />
            {
              <FormControlLabel
                control={
                  <Switch
                    disabled={!fileRenaming}
                    checked={fileRenaming ? autoFileExtension : false}
                    defaultChecked={autoFileExtension}
                    onChange={() => {
                      setAutoFileExtension(state => !state)
                    }}
                  />
                }
                label={i18n.t('autoFileExtensionOption')}
              /> 
            }
            <FormControlLabel
              control={
                <Switch
                  defaultChecked={enableArgs}
                  onChange={() => {
                    setEnableArgs(state => !state)
                  }}
                />
              }
              label={i18n.t('customArgs')}
            />
          </Stack>
        </Grid>
        <Grid sx={{ mr: 1, mt: 2 }}>
          <Typography variant="h6" color="primary" sx={{ mb: 2 }}>
            {i18n.t('credentialsAnchor')}
          </Typography>
          <DownloadCredentials />
        </Grid>

        {/* 网络相关的设置：POT 提供方和代理。
            它们是整个部署的属性，不是某个站点的属性 —— 以前它们是
            站点参数表里的两列，每一行都要填一遍同样的值。 */}
        <Grid sx={{ mr: 1, mt: 2 }}>
          <Typography variant="h6" color="primary" sx={{ mb: 2 }}>
            {i18n.t('networkAnchor')}
          </Typography>
          <NetworkSettings />
        </Grid>

        {/* Cookie 的来源。
            在桌面机器上，浏览器的 cookie 数据库就在本机，直接读它就是当前
            会话；导出文件只是某个时刻的副本。这不是总能成功（Chromium 127
            起把数据库加密到只有浏览器自己能读），所以保存时会实测一次。 */}
        <Grid sx={{ mr: 1, mt: 2 }}>
          <Typography variant="h6" color="primary" sx={{ mb: 2 }}>
            {i18n.t('cookieSourceLabel')}
          </Typography>
          <CookiesFromBrowser />
        </Grid>

        {/* 退出程序。
            桌面版没有控制台也没有自带窗口，浏览器就是它的全部界面 ——
            没有这个按钮，想关掉它只能去任务管理器，而任务管理器不会
            等一个正在写的文件写完。 */}
        <Grid sx={{ mr: 1, mt: 2 }}>
          <Typography variant="h6" color="primary" sx={{ mb: 2 }}>
            {i18n.t('quitButton')}
          </Typography>
          <QuitButton />
        </Grid>

        {/* 反向代理放最后：部署时配置一次，之后基本不动 */}
        <Grid container spacing={2} sx={{ mt: 2 }}>
          <Grid item xs={12}>
            <Typography variant="h6" color="primary" sx={{ mb: 0.5 }}>
              {i18n.t('reverseProxyAnchor')}
            </Typography>
            <FormControlLabel
              control={
                <Checkbox
                  defaultChecked={reverseProxy}
                  onChange={() => setReverseProxy(state => !state)}
                />
              }
              label={i18n.t('servedFromReverseProxyCheckbox')}
              sx={{ mb: 1 }}
            />
            <TextField
              fullWidth
              label={i18n.t('urlBase')}
              defaultValue={baseURL}
              onChange={(e) => {
                let value = e.currentTarget.value
                if (value.startsWith('/')) {
                  value = value.substring(1)
                }
                if (value.endsWith('/')) {
                  value = value.substring(0, value.length - 1)
                }
                baseURL$.next(value)
              }}
              sx={{ mb: 2 }}
            />
          </Grid>
      </Grid>
    </Box>
  )
}

// The settings page.
//
// The content used to be shown in a panel over whatever was on the screen,
// and this is now a destination like Browse and Downloads — so the header
// highlights it when it is open, the browser history moves, and going back
// goes back.
export default function Settings() {
  const { i18n } = useI18n()

  return (
    <Container maxWidth="xl" sx={{ mt: 2, mb: 8 }}>
      <Typography variant="h6" color="primary" sx={{ mb: 2 }}>
        {i18n.t('settingsButtonLabel')}
      </Typography>
      <SettingsContent />
    </Container>
  )
}
