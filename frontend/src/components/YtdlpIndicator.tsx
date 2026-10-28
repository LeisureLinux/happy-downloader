import SystemUpdateAltIcon from '@mui/icons-material/SystemUpdateAlt'
import UpgradeIcon from '@mui/icons-material/Upgrade'
import { Box, CircularProgress, Tooltip } from '@mui/material'
import { useAtomValue } from 'jotai'
import { ytdlpRpcVersionState } from '../atoms/status'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'
import { useRPC } from '../hooks/useRPC'
import { useEffect, useState } from 'react'

// The yt-dlp version, next to the disk figure and the cookie's age.
//
// yt-dlp breaks YouTube roughly monthly, and the fix is a new binary rather
// than a new build of this application. So "which yt-dlp is this" is a
// question worth answering on the page: the version is in the footer, and the
// tooltip says where the binary is, because a host can have several and a
// path is what identifies one.
//
// The version atom is a Promise atom, so reading it needs a Suspense
// boundary; the footer's is the one that provides it.
//
// The upgrade marker appears when there is a newer release. The check is a
// network call, so it is made once and the answer is cached by the server for
// a day.
const YtdlpIndicator: React.FC = () => {
  const version = useAtomValue(ytdlpRpcVersionState)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()
  const { client } = useRPC()

  const [outdated, setOutdated] = useState(false)
  const [latest, setLatest] = useState('')
  const [updating, setUpdating] = useState(false)

  useEffect(() => {
    let cancelled = false

    client.checkYtdlpUpdate()
      .then(res => {
        if (cancelled || !res?.result) return
        setOutdated(res.result.outdated === true)
        setLatest(res.result.latest ?? '')
      })
      .catch(() => {
        // The check failing is not worth a message: the version is still
        // shown, and the marker simply does not appear.
      })

    return () => { cancelled = true }
  }, [client])

  const update = async () => {
    setUpdating(true)

    try {
      await client.updateYtdlp()
      pushMessage(i18n.t('ytdlpUpdated'), 'success')
    } catch (e) {
      pushMessage(String(e), 'error')
    } finally {
      setUpdating(false)
    }
  }

  if (!version.ytdlpVersion || version.ytdlpVersion === 'unknown version') {
    return null
  }

  return (
    <Box
      component="span"
      sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'primary.main' }}
    >
      <Tooltip title={version.ytdlpPath ?? version.ytdlpVersion}>
        <Box component="span" sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <SystemUpdateAltIcon fontSize="small" />
          <Box component="span">{version.ytdlpVersion}</Box>
        </Box>
      </Tooltip>

      {/* The marker is next to the version, not in place of it: the version
          that is running matters too, and it is what the footer is for. */}
      {outdated && (
        <Tooltip title={i18n.t('ytdlpUpdateAvailable').replace('{version}', latest)}>
          <Box
            component="span"
            onClick={update}
            sx={{
              display: 'flex',
              alignItems: 'center',
              cursor: updating ? 'wait' : 'pointer',
              color: 'warning.main',
            }}
          >
            {updating
              ? <CircularProgress size={14} sx={{ color: 'inherit' }} />
              : <UpgradeIcon fontSize="small" />}
          </Box>
        </Tooltip>
      )}
    </Box>
  )
}

export default YtdlpIndicator
