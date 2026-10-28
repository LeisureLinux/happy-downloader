import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Dialog,
  DialogContent,
  IconButton,
  MenuItem,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import DownloadIcon from '@mui/icons-material/Download'
import { base64URLEncode } from '../utils'
import { useI18n } from '../hooks/useI18n'
import { SubtitleFile } from '../types'
import { canPlay, describeCodecs } from '../lib/playback'

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
  // The codecs of the file, as the server read them with ffprobe. Without
  // them the browser is asked to play a file it cannot decode, and shows a
  // black rectangle.
  videoCodec?: string
  audioCodec?: string
  onClose: () => void
}

const Player: React.FC<Props> = ({
  open, serverAddr, path, name, subtitles, videoCodec, audioCodec, onClose
}) => {
  const { i18n } = useI18n()
  const videoRef = useRef<HTMLVideoElement>(null)

  const mediaUrl = useMemo(
    () => `${serverAddr}/filebrowser/v/${base64URLEncode(path)}`,
    [serverAddr, path]
  )

  // The same file, as a download rather than as a stream. It is the answer
  // when the browser cannot play it: the file is served as it is, so a player
  // outside the browser can open it.
  const downloadUrl = useMemo(
    () => `${serverAddr}/filebrowser/d/${base64URLEncode(path)}`,
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

  // Whether this browser can decode this file.
  //
  // The file is not played when it cannot be decoded: a <video> element with
  // an unsupported source shows a black rectangle and says nothing, so the
  // user is left with a screen that looks broken. The codecs come from the
  // server, which read them from the file itself, and the verdict comes from
  // the browser's own canPlayType — the same engine that would do the
  // decoding, so it knows about the hardware as well as the codec.
  const playable = useMemo(
    () => canPlay(name, videoCodec, audioCodec),
    [name, videoCodec, audioCodec]
  )

  const problem = useMemo(
    () => (playable ? '' : describeCodecs(videoCodec, audioCodec)),
    [playable, videoCodec, audioCodec]
  )

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
        {playable ? (
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
        ) : (
          <Box sx={{ p: 3 }}>
            <Alert severity="warning">
              <AlertTitle>{i18n.t('playbackUnsupported')}</AlertTitle>
              <Typography variant="body2">
                {/* The codec is named twice in the message — what the file
                    uses, and what a player has to support — so every
                    occurrence is replaced, not only the first. */}
                {i18n.t('playbackUnsupportedBody')
                  .split('{codecs}').join(problem)
                  .split('{name}').join(name)}
              </Typography>

              {/* The message says "download it", so the button is right
                  there. The file is served as it is, which is what a player
                  outside the browser needs. */}
              <Button
                variant="outlined"
                startIcon={<DownloadIcon />}
                component="a"
                href={downloadUrl}
                download={name}
                sx={{ mt: 2 }}
              >
                {i18n.t('downloadFile')}
              </Button>
            </Alert>
          </Box>
        )}
      </DialogContent>
    </Dialog>
  )
}

export default Player
