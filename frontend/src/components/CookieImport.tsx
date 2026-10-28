import UploadFileIcon from '@mui/icons-material/UploadFile'
import { Alert, Box, Button, Paper, Stack, Typography } from '@mui/material'
import { useRef, useState } from 'react'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'

// Importing a cookie export.
//
// The automatic export belongs to whoever set up the host: a browser profile
// read by the refresh script and copied over by a timer. The person using
// the interface may well want their own session instead — the one they are
// signed into on their own machine.
//
// The file is chosen from the browser, so it never leaves the user's machine
// except as the upload to this server. The format is recognised from the
// content: the JSON that a browser extension produces and a Netscape
// cookies.txt are both accepted.
//
// While it is there it is the cookie the downloads use. After two refusals
// the server removes it and the automatic export takes over again, so this
// panel has to say which one is in effect.
const CookieImport: React.FC<{ onDone: () => void }> = ({ onDone }) => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const inputRef = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const upload = async (file: File) => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/import`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/octet-stream' },
        body: file,
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const result = await res.json()

      pushMessage(
        i18n.t('cookieImported').replace('{count}', String(result.count ?? 0)),
        'success'
      )

      onDone()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const remove = async () => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/cookies/import`, { method: 'DELETE' })
      if (!res.ok) throw new Error(await res.text())

      pushMessage(i18n.t('cookieImportRemoved'), 'info')
      onDone()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Paper variant="outlined" sx={{ p: 1.5 }}>
      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
        <Typography variant="body2" sx={{ flexGrow: 1 }}>
          {i18n.t('cookieImportHint')}
        </Typography>

        <Button
          size="small"
          variant="outlined"
          startIcon={<UploadFileIcon />}
          disabled={busy}
          onClick={() => inputRef.current?.click()}
        >
          {i18n.t('cookieImportButton')}
        </Button>

        <Button
          size="small"
          color="error"
          disabled={busy}
          onClick={remove}
        >
          {i18n.t('cookieImportRemove')}
        </Button>

        {/* The file input is hidden: the button opens the file chooser, and
            a visible input would show the full path and a second control for
            the same thing. */}
        <input
          ref={inputRef}
          type="file"
          accept=".json,.txt,application/json,text/plain"
          hidden
          onChange={e => {
            const file = e.target.files?.[0]
            // Reset, so choosing the same file twice fires the change
            // event again.
            e.target.value = ''
            if (file) void upload(file)
          }}
        />
      </Stack>

      {error && (
        <Alert severity="error" sx={{ mt: 1 }}>
          <Box component="span">{error}</Box>
        </Alert>
      )}
    </Paper>
  )
}

export default CookieImport
