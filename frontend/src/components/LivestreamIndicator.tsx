import SensorsIcon from '@mui/icons-material/Sensors'
import StopCircleIcon from '@mui/icons-material/StopCircle'
import { Box, IconButton, Stack, Tooltip, Typography } from '@mui/material'
import { useEffect, useState } from 'react'
import { useRPC } from '../hooks/useRPC'
import { useI18n } from '../hooks/useI18n'
import { LiveStreamProgress } from '../types'

// The livestreams that are being watched for or downloaded.
//
// A livestream does not end, so it needs a way to be stopped — and it needs
// to say that it is running, because the queue shows it as "preparing" for as
// long as the stream lasts.
//
// The panel is in the footer for the same reason the log is: both are things
// you watch while something else happens.
const LivestreamIndicator: React.FC = () => {
  const { client } = useRPC()
  const { i18n } = useI18n()

  const [streams, setStreams] = useState<LiveStreamProgress>({})

  useEffect(() => {
    let cancelled = false

    const poll = async () => {
      try {
        const res = await client.progressLivestream()
        if (!cancelled && res?.result) {
          setStreams(res.result)
        }
      } catch {
        // The poll failing is not worth a message: the next one will do.
      }
    }

    void poll()
    const timer = setInterval(poll, 3000)

    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [client])

  const urls = Object.keys(streams)
  if (urls.length === 0) return null

  return (
    <Stack direction="row" spacing={0.5} alignItems="center">
      {urls.map(url => (
        <Tooltip key={url} title={`${i18n.t('livestreamRunning')} · ${url}`}>
          <Stack direction="row" spacing={0.25} alignItems="center">
            <SensorsIcon fontSize="small" sx={{ color: 'primary.main' }} />
            <Typography variant="caption" color="text.secondary" noWrap
              sx={{ maxWidth: 120 }}>
              {url.split('/').pop()}
            </Typography>
            <IconButton
              size="small"
              onClick={() => { void client.killLivestream(url) }}
              aria-label={i18n.t('livestreamStop')}
            >
              <StopCircleIcon fontSize="small" sx={{ color: 'warning.main' }} />
            </IconButton>
          </Stack>
        </Tooltip>
      ))}
    </Stack>
  )
}

export default LivestreamIndicator
