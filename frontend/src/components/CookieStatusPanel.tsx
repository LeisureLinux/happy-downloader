import { Suspense, useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Chip,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import { cookieStatusState, serverURL, type CookieStatus } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useAtomValue } from 'jotai'

// 只读状态面板：cookie 文件由 paths.cookie_path 指定，服务端自动转换并监听改动，
// 所以这里不需要任何编辑入口。
const CookieStatusPanel: React.FC = () => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()

  const initial = useAtomValue(cookieStatusState)

  const [status, setStatus] = useState<CookieStatus>(initial)
  const [busy, setBusy] = useState(false)

  const refresh = useCallback(async () => {
    setBusy(true)
    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/status`, {
        headers: { 'X-Authentication': localStorage.getItem('token') ?? '' }
      })
      if (res.ok) setStatus(await res.json())
    } catch {
      // 状态是附加信息，取不到不影响下载
    } finally {
      setBusy(false)
    }
  }, [serverAddr])

  useEffect(() => {
    void refresh()
  }, [refresh])

  if (busy && !status) {
    return <CircularProgress size={20} />
  }

  // 没有配置路径：明确告诉使用者该改哪个配置项
  if (!status.exists && !status.sourcePath) {
    return (
      <Alert severity="info" icon={false}>
        <Typography variant="body2">{i18n.t('cookieNotConfigured')}</Typography>
        <Typography variant="caption" component="pre" sx={{ mt: 1, whiteSpace: 'pre-wrap' }}>
          {'paths:\n  cookie_path: /path/to/xxx-cookie.json'}
        </Typography>
      </Alert>
    )
  }

  const missing = status.missing ?? []
  const hasLogin = missing.length === 0

  return (
    <Stack spacing={1}>
      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
        <Chip
          size="small"
          label={status.exists ? i18n.t('cookieLoaded') : i18n.t('cookieNotLoaded')}
          color={status.exists ? (hasLogin ? 'success' : 'warning') : 'default'}
        />
        {status.format && (
          <Chip size="small" variant="outlined" label={status.format} />
        )}
        <Typography variant="body2">
          {i18n.t('cookieCount')}: {status.count}
        </Typography>
        {!!status.httpOnly && (
          <Typography variant="caption" color="text.secondary">
            httpOnly {status.httpOnly}
          </Typography>
        )}
        {!!status.expired && (
          <Typography variant="caption" color="error.main">
            {i18n.t('cookieExpired')}: {status.expired}
          </Typography>
        )}
        <Tooltip title={i18n.t('cookieRefreshStatus')}>
          <IconButton size="small" onClick={refresh} disabled={busy}>
            {busy ? <CircularProgress size={16} /> : <RefreshIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
      </Stack>

      {/* 路径可见：使用者要 scp 到哪一目了然 */}
      {status.sourcePath && (
        <Typography variant="caption" component="pre" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
          {status.sourcePath}
        </Typography>
      )}

      {status.exists && !hasLogin && (
        <Alert severity="warning" icon={false}>
          <Typography variant="body2">{i18n.t('cookieMissingLogin')}</Typography>
          <Typography variant="caption">{missing.join(', ')}</Typography>
        </Alert>
      )}

      <Typography variant="caption" color="text.secondary">
        {i18n.t('cookieAutoDetectHint')}
      </Typography>
    </Stack>
  )
}

const CookieStatusWithSuspense: React.FC = () => (
  <Suspense fallback={<CircularProgress size={20} />}>
    <CookieStatusPanel />
  </Suspense>
)

export default CookieStatusWithSuspense
