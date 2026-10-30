import SaveIcon from '@mui/icons-material/Save'
import {
  Alert,
  Box,
  Button,
  Paper,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useCallback, useEffect, useState } from 'react'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'

// The settings that belong to the installation rather than to a site.
//
// The PO Token provider and the proxy used to be fields in the profile table,
// which meant the same value was typed into every row of it — three settings
// and a site each, for two values that never differed.
//
// They are here instead: one provider, one proxy, and the sites that need
// one are worked out when a download is built.
//
// The proxy in particular is not a row any more. It was the fallback for every
// site a profile did not name, so a proxy that was there for YouTube was also
// sent to Bilibili — which answers a request from another country with a
// block rather than with the video. The route is now decided per site: the
// ones that are known to need the proxy get it, and the rest are reached
// directly, with the answer remembered.

type NetworkConfig = {
  potBaseUrl: string
  proxy: string
  downloadPath: string
  defaultDownloadPath: string
}

const NetworkSettings: React.FC = () => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [pot, setPot] = useState('')
  const [proxy, setProxy] = useState('')
  const [downloadPath, setDownloadPath] = useState('')
  const [defaultDownloadPath, setDefaultDownloadPath] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch(`${serverAddr}/api/v1/network`)
      if (!res.ok) return

      const config = await res.json() as NetworkConfig
      setPot(config.potBaseUrl ?? '')
      setProxy(config.proxy ?? '')
      setDownloadPath(config.downloadPath ?? '')
      setDefaultDownloadPath(config.defaultDownloadPath ?? '')
    } catch {
      // The settings not loading is not worth a message: the form is still
      // usable, and saving it would write what is in it.
    }
  }, [serverAddr])

  useEffect(() => { void load() }, [load])

  const save = async () => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/network`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ potBaseUrl: pot, proxy, downloadPath }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      pushMessage(i18n.t('networkSaved'), 'success')
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Paper variant="outlined" sx={{ p: 1.5 }}>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
        {i18n.t('networkSettingsHint')}
      </Typography>

      <Stack spacing={2}>
        {/* Where the files go. It is the first setting on the page because
            it is the one people come here to change: the default used to be
            the folder the program was started from, which on a desktop is
            the folder it was unpacked into. */}
        <Box>
          <TextField
            size="small"
            fullWidth
            label={i18n.t('downloadPathLabel')}
            placeholder={defaultDownloadPath}
            helperText={i18n.t('downloadPathHint')}
            value={downloadPath}
            onChange={e => setDownloadPath(e.target.value)}
          />
          {defaultDownloadPath && (
            <Button
              size="small"
              sx={{ mt: 0.5 }}
              disabled={busy || downloadPath === defaultDownloadPath}
              onClick={() => setDownloadPath(defaultDownloadPath)}
            >
              {i18n.t('useDefaultDownloadPath')}
            </Button>
          )}
        </Box>

        <TextField
          size="small"
          fullWidth
          label={i18n.t('potProviderLabel')}
          placeholder="http://127.0.0.1:4416"
          helperText={i18n.t('potProviderHint')}
          value={pot}
          onChange={e => setPot(e.target.value)}
        />

        <TextField
          size="small"
          fullWidth
          label={i18n.t('proxyLabel')}
          placeholder="http://127.0.0.1:8888"
          helperText={i18n.t('proxyHint')}
          value={proxy}
          onChange={e => setProxy(e.target.value)}
        />

        <Box>
          <Button
            size="small"
            variant="outlined"
            startIcon={<SaveIcon />}
            disabled={busy}
            onClick={save}
          >
            {i18n.t('saveButton')}
          </Button>
        </Box>

        {error && (
          <Alert severity="error">
            <Box component="span">{error}</Box>
          </Alert>
        )}
      </Stack>
    </Paper>
  )
}

export default NetworkSettings
