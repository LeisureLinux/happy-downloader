import { useCallback, useEffect, useRef, useState } from 'react'
import { Button, Container, Stack, Typography, styled } from '@mui/material'
import CloudOffIcon from '@mui/icons-material/CloudOff'
import { useNavigate } from 'react-router-dom'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useAtomValue } from 'jotai'

const FlexContainer = styled(Container)({
  display: 'flex',
  minWidth: '100%',
  minHeight: '70vh',
  alignItems: 'center',
  justifyContent: 'center',
  flexDirection: 'column',
  gap: '0.75rem'
})

// Shown when the server cannot be reached, most often because it is
// restarting. It returns to the home page on its own after a moment: the
// failure is usually transient, and a page that only offers "go to settings"
// leaves the user stranded on an error screen after the server is already
// back. The manual button is there for the case where it is not transient.
const RETRY_MS = 1000

const ErrorBoundary: React.FC = () => {
  const navigate = useNavigate()
  const { i18n } = useI18n()
  const serverAddr = useAtomValue(serverURL)

  const [attempt, setAttempt] = useState(0)
  const cancelled = useRef(false)

  // Return home only once the server answers. Navigating blind would fail on
  // the lazily loaded page chunks, which are served by the same server that is
  // down, and land the user right back here.
  const probeAndReturn = useCallback(async () => {
    try {
      const res = await fetch(`${serverAddr}/api/v1/version`, { cache: 'no-store' })
      if (cancelled.current) return
      if (res.ok) {
        navigate('/', { replace: true })
        return
      }
    } catch {
      // still down
    }
    if (!cancelled.current) setAttempt(a => a + 1)
  }, [navigate, serverAddr])

  useEffect(() => {
    cancelled.current = false
    const t = setTimeout(() => { void probeAndReturn() }, RETRY_MS)
    return () => {
      cancelled.current = true
      clearTimeout(t)
    }
  }, [attempt, probeAndReturn])

  return (
    <FlexContainer>
      <CloudOffIcon sx={{ fontSize: 72, color: 'text.disabled' }} />

      <Typography variant="h5" color="text.secondary">
        {i18n.t('connectionLostTitle')}
      </Typography>
      <Typography variant="body2" color="text.disabled">
        {i18n.t('connectionLostHint')}
      </Typography>
      {attempt > 0 && (
        <Typography variant="caption" color="text.disabled">
          {i18n.t('retrying')} ({attempt})
        </Typography>
      )}

      <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
        <Button variant="contained" onClick={() => navigate('/', { replace: true })}>
          {i18n.t('goHome')}
        </Button>
        <Button onClick={() => navigate('/settings')}>
          {i18n.t('settingsButtonLabel')}
        </Button>
      </Stack>
    </FlexContainer>
  )
}

export default ErrorBoundary
