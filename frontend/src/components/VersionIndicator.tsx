import { useState } from 'react'
import { Box, Chip, CircularProgress, IconButton, Tooltip } from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import { ytdlpRpcVersionState } from '../atoms/status'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'
import { useRPC } from '../hooks/useRPC'
import { useAtomValue } from 'jotai'

// Version chips plus the yt-dlp upgrade action, kept together because the
// version shown is the thing being upgraded.
//
// The UI version used to be a hardcoded string that silently drifted; it now
// comes from the server, which reports the version it was built with.
const VersionIndicator: React.FC = () => {
  const version = useAtomValue(ytdlpRpcVersionState)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()
  const { client } = useRPC()

  const [checking, setChecking] = useState(false)

  // yt-dlp's own --version already tells us what is running, so the latest
  // release is only worth a round trip when the operator asks for it.
  const update = async () => {
    setChecking(true)
    try {
      const before = version.ytdlpVersion
      await client.updateExecutable()

      const after = await client.version()
      const current = (after?.result as { ytdlpVersion?: string })?.ytdlpVersion ?? before

      pushMessage(
        current && current !== before
          ? `${i18n.t('ytdlpUpdatedTo')} ${current}`
          : i18n.t('ytdlpAlreadyLatest'),
        'success'
      )
    } catch (e) {
      pushMessage(String(e), 'error')
    } finally {
      setChecking(false)
    }
  }

  return (
    <Box style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
      <Chip
        label={`UI v${version.rpcVersion}`}
        variant="outlined"
        size="small"
      />
      <Chip label={`yt-dlp v${version.ytdlpVersion}`} variant="outlined" size="small" />
      <Tooltip title={i18n.t('updateBinButton')}>
        <IconButton size="small" onClick={update} disabled={checking}>
          {checking ? <CircularProgress size={14} /> : <RefreshIcon fontSize="small" />}
        </IconButton>
      </Tooltip>
    </Box>
  )
}

export default VersionIndicator
