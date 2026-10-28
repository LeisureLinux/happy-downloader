import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import RefreshIcon from '@mui/icons-material/Refresh'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'

// One site's argument set. Mirrors the server's Profile struct.
// One site's arguments.
//
// The cookie, the PO Token provider and the proxy are not here: they are
// properties of the installation rather than of a site, and they were three
// settings that had to be typed into every row of a table. What is left is
// what genuinely differs per site — the extra arguments.
type Profile = {
  name: string
  hostSuffix: string
  extraArgs?: string[]
  preferFreeFormats?: boolean
}

type CookieStat = {
  exists: boolean
  sourcePath?: string
  format?: string
  count: number
  httpOnly?: number
  session?: number
  expired?: number
  missing?: string[]
  modifiedAt?: string
  // Which cookie the downloads are using: "imported" or "automatic".
  active?: string
  // The export the user uploaded, when there is one.
  imported?: {
    path: string
    format: string
    count: number
    modifiedAt: string
    failures: number
  }
}

type Preview = {
  url: string
  matched: boolean
  profile: string
  argv: string[]
}

const emptyProfile = (): Profile => ({ name: '', hostSuffix: '', extraArgs: [] })

const DownloadCredentials: React.FC = () => {
  // serverURL is an atom; read its value. Using the atom object
  // directly stringifies to "[object Object]", so every request 404s and
  // the status stays null, which re-renders in a loop.
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [profiles, setProfiles] = useState<Profile[]>([])
  const [stat, setStat] = useState<CookieStat | null>(null)
  const [busy, setBusy] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [previewUrl, setPreviewUrl] = useState('')
  const [preview, setPreview] = useState<Preview | null>(null)

  // useToast() returns a fresh object every render, so pushMessage is a new
  // function reference each time. Putting it in a dependency array would
  // recreate refresh() on every render, and the useEffect below would re-fire
  // in a loop — each setStat() triggering another render. Hold it in a ref
  // instead so refresh() is stable.
  const notifyRef = useRef(pushMessage)
  notifyRef.current = pushMessage

  const refresh = useCallback(async () => {
    setBusy(true)
    try {
      const [p, s] = await Promise.all([
        fetch(`${serverAddr}/api/v1/profiles/`)
          .then(r => r.ok ? r.json() : []),
        fetch(`${serverAddr}/api/v1/cookies/status`)
          .then(r => r.ok ? r.json() : null)
      ])
      setProfiles(p)
      setStat(s)
      setDirty(false)
    } catch (e) {
      notifyRef.current(String(e), 'error')
    } finally {
      setBusy(false)
    }
  }, [serverAddr])

  // 只在 serverAddr 变化时拉一次，不在每次渲染后重复请求
  useEffect(() => { void refresh() }, [refresh])

  const save = useCallback(async () => {
    setBusy(true)
    try {
      const res = await fetch(`${serverAddr}/api/v1/profiles/`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(profiles)
      })
      if (!res.ok) {
        notifyRef.current(await res.text(), 'error')
        return
      }
      setProfiles(await res.json())
      setDirty(false)
      notifyRef.current(i18n.t('profilesSaved'), 'success')
    } catch (e) {
      notifyRef.current(String(e), 'error')
    } finally {
      setBusy(false)
    }
  }, [profiles, serverAddr, i18n])

  const runPreview = useCallback(async (u: string) => {
    if (!u.trim()) { setPreview(null); return }
    try {
      const res = await fetch(
        `${serverAddr}/api/v1/profiles/preview?url=${encodeURIComponent(u)}`)
      if (res.ok) setPreview(await res.json())
    } catch {
      setPreview(null)
    }
  }, [serverAddr])

  const patch = (idx: number, field: keyof Profile, value: unknown) => {
    setProfiles(ps => ps.map((p, i) => (i === idx ? { ...p, [field]: value } : p)))
    setDirty(true)
  }

  const add = () => { setProfiles(ps => [...ps, emptyProfile()]); setDirty(true) }
  const remove = (idx: number) => { setProfiles(ps => ps.filter((_, i) => i !== idx)); setDirty(true) }

  const missing = stat?.missing ?? []
  const hasLogin = !stat?.exists || missing.length === 0

  return (
    <Stack spacing={2}>
      <Typography variant="body2" color="text.secondary">
        {i18n.t('profilesHint')}
      </Typography>

      {/* ── 当前生效的 cookie 文件 ─────────────────────────────
          只有路径和刷新按钮。数量、格式、最后修改时间都挪走了：时间和磁盘
          空间一样是常驻状态，放在左下角一直能看见；数量和格式是排障时才看的
          东西，而这个面板要打开才看得见，留着只是把路径挤小。 */}
      <Paper variant="outlined" sx={{ p: 1.5 }}>
        <Stack direction="row" spacing={1} alignItems="flex-start">
          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            <Typography variant="caption" color="text.secondary">
              {stat?.active === 'imported'
                ? i18n.t('cookieImportedPath')
                : i18n.t('cookieSourcePath')}
            </Typography>
            <Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
              {stat?.active === 'imported'
                ? (stat?.imported?.path || '—')
                : (stat?.sourcePath || '—')}
            </Typography>
          </Box>
          <Tooltip title={i18n.t('cookieRefreshStatus')}>
            <IconButton size="small" onClick={refresh} disabled={busy}>
              {busy ? <CircularProgress size={16} /> : <RefreshIcon fontSize="small" />}
            </IconButton>
          </Tooltip>
        </Stack>
      </Paper>

      {stat?.exists && !hasLogin && (
        <Alert severity="warning" icon={false}>
          <Typography variant="body2">{i18n.t('cookieMissingLogin')}</Typography>
          <Typography variant="caption">{missing.join(', ')}</Typography>
        </Alert>
      )}

      {/* ── 站点参数表 ─────────────────────────────────────── */}
      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{i18n.t('profileName')}</TableCell>
              <TableCell>{i18n.t('profileHost')}</TableCell>
              <TableCell>{i18n.t('profileExtraArgs')}</TableCell>
              <TableCell width={48} />
            </TableRow>
          </TableHead>
          <TableBody>
            {profiles.map((p, idx) => (
              <TableRow key={idx} sx={{ verticalAlign: "top" }}>
                <TableCell>
                  <TextField
                    size="small"
                    fullWidth
                    value={p.name}
                    onChange={e => patch(idx, 'name', e.target.value)}
                  />
                </TableCell>
                <TableCell>
                  <TextField
                    size="small"
                    fullWidth
                    placeholder="youtube.com"
                    value={p.hostSuffix}
                    onChange={e => patch(idx, 'hostSuffix', e.target.value)}
                  />
                  {p.hostSuffix === '' && (
                    <Typography variant="caption" color="warning.main">
                      {i18n.t('profileFallbackNote')}
                    </Typography>
                  )}
                </TableCell>
                <TableCell>
                  <TextField
                    size="small"
                    fullWidth
                    multiline
                    minRows={1}
                    placeholder={'--sub-langs en,zh-Hans\n--embed-thumbnail'}
                    value={(p.extraArgs ?? []).join('\n')}
                    onChange={e =>
                      patch(idx, 'extraArgs',
                        e.target.value.split('\n').map(s => s.trim()).filter(Boolean))
                    }
                  />
                </TableCell>
                <TableCell>
                  <IconButton size="small" onClick={() => remove(idx)} disabled={busy}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      <Stack direction="row" spacing={1}>
        <Button size="small" startIcon={<AddIcon />} onClick={add} disabled={busy}>
          {i18n.t('profileAdd')}
        </Button>
        <Button
          size="small"
          variant="contained"
          onClick={save}
          disabled={busy || !dirty}
        >
          {i18n.t('profilesSave')}
        </Button>
        <Box flexGrow={1} />
        {dirty && <Typography variant="caption" color="warning.main">{i18n.t('profilesUnsaved')}</Typography>}
      </Stack>

      {/* ── argv 预览：确认实际会传什么，便于排查 ──────────── */}
      <Paper variant="outlined" sx={{ p: 1.5 }}>
        <Typography variant="subtitle2" sx={{ mb: 1 }}>
          {i18n.t('previewAnchor')}
        </Typography>
        <Stack direction="row" spacing={1} alignItems="center">
          <TextField
            size="small"
            fullWidth
            placeholder="https://www.youtube.com/watch?v=..."
            value={previewUrl}
            onChange={e => { setPreviewUrl(e.target.value); void runPreview(e.target.value) }}
          />
          {preview && (
            <Chip
              size="small"
              label={preview.matched ? preview.profile : i18n.t('profileFallbackNote')}
              color={preview.matched ? 'primary' : 'default'}
              variant="outlined"
            />
          )}
        </Stack>
        {preview && (
          <Box
            component="pre"
            sx={{
              mt: 1, p: 1, mb: 0, borderRadius: 1, overflowX: 'auto',
              bgcolor: 'action.hover', fontSize: '11.5px', fontFamily: 'Roboto Mono',
              whiteSpace: 'pre-wrap', wordBreak: 'break-all'
            }}
          >
            {preview.argv.join(' ')}
          </Box>
        )}
        <Typography variant="caption" color="text.secondary">
          {i18n.t('previewHint')}
        </Typography>
      </Paper>

      <Alert severity="info" icon={false}>
        <Typography variant="caption">{i18n.t('profilesIgnoreConfigNote')}</Typography>
      </Alert>
    </Stack>
  )
}

export default DownloadCredentials
