import {
  Alert,
  Box,
  Button,
  MenuItem,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useCallback, useEffect, useState } from 'react'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'

// Where the signed-in session comes from.
//
// A cookie export is a photograph: taken at one moment and kept until somebody
// remembers to take another. The browser's own database is the thing that
// changes, so a download next week still has next week's session — which is
// why this is offered at all.
//
// It does not always work. Chromium browsers since version 127 encrypt the
// database with a key bound to the browser process, and nothing outside the
// browser can read it. That is not something this program can work around, so
// the answer is checked when the setting is saved and the result is shown:
// accepting a setting that does not work turns into a download that fails for
// no stated reason.

type ProbeResult = {
  ok: boolean
  cookies: number
  problem?: string
  detail?: string
}

type Config = {
  fromBrowser: string
  choices: string[]
}

const CookiesFromBrowser: React.FC = () => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()

  const [fromBrowser, setFromBrowser] = useState('')
  const [choices, setChoices] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<ProbeResult | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/browser`)
      if (!res.ok) return

      const config = await res.json() as Config
      setFromBrowser(config.fromBrowser ?? '')
      setChoices(config.choices ?? [])
    } catch {
      // The form is still usable if the settings cannot be read; saving would
      // write what is in it.
    }
  }, [serverAddr])

  useEffect(() => { void load() }, [load])

  const save = async (value: string) => {
    setBusy(true)
    setError('')
    setResult(null)

    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/browser`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fromBrowser: value }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      setFromBrowser(value)
      setResult(await res.json() as ProbeResult)
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const message = (r: ProbeResult) => {
    switch (r.problem) {
      case 'encrypted':
        return i18n.t('cookieBrowserEncrypted')
      case 'locked':
        return i18n.t('cookieBrowserLocked')
      case 'unreadable':
        return i18n.t('cookieBrowserUnreadable')
      default:
        return i18n.t('cookieBrowserUnknown')
    }
  }

  return (
    <Stack spacing={1.5}>
      <Typography variant="body2" color="text.secondary">
        {i18n.t('cookieSourceHint')}
      </Typography>

      <TextField
        select
        size="small"
        fullWidth
        label={i18n.t('cookieSourceLabel')}
        value={fromBrowser}
        disabled={busy}
        onChange={e => void save(e.target.value)}
        helperText={i18n.t('cookieSourceFileHint')}
      >
        <MenuItem value="">{i18n.t('cookieSourceFile')}</MenuItem>
        {choices.map(choice => (
          <MenuItem key={choice} value={choice}>
            {choice}
          </MenuItem>
        ))}
      </TextField>

      {fromBrowser !== '' && (
        <Box>
          <Button size="small" variant="outlined" disabled={busy} onClick={() => void save(fromBrowser)}>
            {i18n.t('cookieSourceTest')}
          </Button>
        </Box>
      )}

      {error && <Alert severity="error"><Box component="span">{error}</Box></Alert>}

      {result && (
        result.ok ? (
          <Alert severity="success">
            {i18n.t('cookieBrowserOK')} {result.cookies}
          </Alert>
        ) : (
          <Alert severity="warning">
            <Box component="span">{message(result)}</Box>
            {result.detail && (
              <Box
                component="pre"
                sx={{
                  mt: 1,
                  mb: 0,
                  fontSize: 11,
                  maxHeight: 120,
                  overflow: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {result.detail}
              </Box>
            )}
          </Alert>
        )
      )}
    </Stack>
  )
}

export default CookiesFromBrowser
