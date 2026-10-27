import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Box,
  Dialog,
  DialogContent,
  IconButton,
  MenuItem,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { base64URLEncode } from '../utils'
import { useI18n } from '../hooks/useI18n'
import { SubtitleFile } from '../types'

// The player.
//
// It replaces window.open on a media file, which handed the video to the
// browser's own player. That one has no way to offer a caption track, and the
// whole point of downloading captions is being able to turn them on.
//
// The captions are ordinary <track> elements, so the browser does the
// rendering and the subtitle menu is the browser's. Which track is available
// comes from the server, which looks for the caption files next to the video.

type Props = {
  open: boolean
  serverAddr: string
  path: string
  name: string
  subtitles?: SubtitleFile[]
  onClose: () => void
}

const Player: React.FC<Props> = ({ open, serverAddr, path, name, subtitles, onClose }) => {
  const { i18n } = useI18n()
  const videoRef = useRef<HTMLVideoElement>(null)

  const mediaUrl = useMemo(
    () => `${serverAddr}/filebrowser/v/${base64URLEncode(path)}`,
    [serverAddr, path]
  )

  const tracks = useMemo(
    () => (subtitles ?? []).map(s => ({
      ...s,
      src: `${serverAddr}/filebrowser/s/${base64URLEncode(s.path)}`,
    })),
    [serverAddr, subtitles]
  )

  // Which caption track is on. The browser remembers the choice per element,
  // but the menu has to show it, so it is kept here as well.
  const [track, setTrack] = useState('')

  useEffect(() => {
    if (open) {
      setTrack('')
    }
  }, [open, path])

  const applyTrack = (language: string) => {
    setTrack(language)

    const video = videoRef.current
    if (!video) return

    for (const t of Array.from(video.textTracks)) {
      t.mode = t.language === language ? 'showing' : 'disabled'
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="lg"
      fullWidth
      PaperProps={{ sx: { bgcolor: 'background.paper' } }}
    >
      <Stack direction="row" spacing={1} alignItems="center" sx={{ px: 2, py: 1 }}>
        <Typography variant="subtitle1" noWrap sx={{ flexGrow: 1 }}>
          {name}
        </Typography>

        {tracks.length > 0 && (
          <TextField
            select
            size="small"
            label={i18n.t('subtitlesLabel')}
            value={track}
            onChange={e => applyTrack(e.target.value)}
            sx={{ minWidth: 170 }}
          >
            <MenuItem value="">{i18n.t('captionsOff')}</MenuItem>
            {tracks.map(t => (
              <MenuItem key={t.path} value={t.language}>
                {t.label}
              </MenuItem>
            ))}
          </TextField>
        )}

        <IconButton size="small" onClick={onClose} aria-label={i18n.t('close')}>
          <CloseIcon fontSize="small" />
        </IconButton>
      </Stack>

      <DialogContent sx={{ p: 0, bgcolor: 'black' }}>
        <Box
          component="video"
          ref={videoRef}
          src={mediaUrl}
          controls
          autoPlay
          sx={{ width: '100%', maxHeight: '75vh', display: 'block' }}
        >
          {tracks.map(t => (
            <track
              key={t.path}
              kind="subtitles"
              src={t.src}
              srcLang={t.language}
              label={t.label}
            />
          ))}
        </Box>
      </DialogContent>
    </Dialog>
  )
}

export default Player
