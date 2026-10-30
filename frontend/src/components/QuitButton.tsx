import { Alert, Box, Button, Stack, Typography } from '@mui/material'
import PowerSettingsNewIcon from '@mui/icons-material/PowerSettingsNew'
import { useState } from 'react'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'

// Stopping the application from the page.
//
// On a desktop install there is no terminal to press Ctrl-C in and no window
// with a close button: the program starts, opens a browser, and the browser is
// all there is of it. Without this, stopping it means finding it in the task
// manager — which is not a way to stop a program that may be half way through
// writing a file.
//
// It is a two-step button, because it is the one control on this page whose
// effect cannot be undone by staying on the page.

const QuitButton: React.FC = () => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()

  const [asking, setAsking] = useState(false)
  const [stopping, setStopping] = useState(false)
  const [error, setError] = useState('')

  const quit = async () => {
    setStopping(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/quit`, { method: 'POST' })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      // The server stops a moment after it answers, so there is nothing left
      // to wait for and nothing left to talk to. The page says so rather
      // than looking like a failed request.
      setAsking(false)
    } catch (e) {
      setError(String(e))
      setStopping(false)
    }
  }

  return (
    <Stack spacing={1}>
      <Typography variant="body2" color="text.secondary">
        {i18n.t('quitHint')}
      </Typography>

      {error && <Alert severity="error"><Box component="span">{error}</Box></Alert>}

      {stopping ? (
        <Alert severity="warning">{i18n.t('quitStopped')}</Alert>
      ) : asking ? (
        <Stack direction="row" spacing={1}>
          <Button
            size="small"
            variant="contained"
            color="error"
            startIcon={<PowerSettingsNewIcon />}
            onClick={quit}
          >
            {i18n.t('quitConfirm')}
          </Button>
          <Button size="small" onClick={() => setAsking(false)}>
            {i18n.t('cancel')}
          </Button>
        </Stack>
      ) : (
        <Box>
          <Button
            size="small"
            variant="outlined"
            color="error"
            startIcon={<PowerSettingsNewIcon />}
            onClick={() => setAsking(true)}
          >
            {i18n.t('quitButton')}
          </Button>
        </Box>
      )}
    </Stack>
  )
}

export default QuitButton
