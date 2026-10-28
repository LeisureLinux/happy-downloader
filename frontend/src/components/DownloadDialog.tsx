import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Stack,
  Typography
} from '@mui/material'
import DownloadIcon from '@mui/icons-material/Download'
import MovieIcon from '@mui/icons-material/Movie'
import SubtitleIcon from '@mui/icons-material/Subtitles'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'

// What can be downloaded for one file.
//
// The video and each of its caption files are separate downloads, and this
// dialog lists them separately.
//
// It used to be one button that produced a zip containing the video and its
// captions. On a phone that is a poor answer: the video arrives as a .zip
// that nothing can play, and unzipping it there is a chore. A person who
// wanted the video to watch on the phone had to unpack an archive first.
//
// So each file is its own download here, and the captions are a list of
// choices rather than something bundled behind the video.

type Downloadable = {
  path: string
  name: string
  url: string
  kind: string
  language?: string
  label?: string
  size: number
}

type Props = {
  open: boolean
  path: string
  name: string
  onClose: () => void
}

const DownloadDialog: React.FC<Props> = ({ open, path, name, onClose }) => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()

  const [files, setFiles] = useState<Downloadable[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')

    try {
      const res = await fetch(
        `${serverAddr}/filebrowser/downloadable?path=${encodeURIComponent(path)}`)

      if (!res.ok) {
        throw new Error(await res.text())
      }

      setFiles(await res.json())
    } catch (e) {
      setError(String(e))
      setFiles([])
    } finally {
      setLoading(false)
    }
  }, [serverAddr, path])

  useEffect(() => {
    if (open) {
      void load()
    }
  }, [open, load])

  const video = files.find(f => f.kind === 'video')
  const captions = files.filter(f => f.kind === 'caption')

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Stack direction="row" spacing={1} alignItems="center">
          <DownloadIcon />
          <span>{i18n.t('downloadDialogTitle')}</span>
        </Stack>
        <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
          {name}
        </Typography>
      </DialogTitle>

      <DialogContent>
        {loading && (
          <Stack direction="row" spacing={1} alignItems="center" sx={{ py: 2 }}>
            <CircularProgress size={16} />
            <span>{i18n.t('loadingLabel')}</span>
          </Stack>
        )}

        {error && <Alert severity="error">{error}</Alert>}

        {!loading && !error && (
          <List dense disablePadding>
            {/* The video first: it is what the person came for. */}
            {video && (
              <ListItem disableGutters>
                <ListItemIcon><MovieIcon fontSize="small" /></ListItemIcon>
                <ListItemText
                  primary={video.name}
                  secondary={i18n.t('downloadVideoOnly')}
                />
                <Button
                  size="small"
                  variant="outlined"
                  component="a"
                  href={video.url}
                  download={video.name}
                >
                  {i18n.t('downloadFile')}
                </Button>
              </ListItem>
            )}

            {/* The captions, one button each. */}
            {captions.map(c => (
              <ListItem key={c.path} disableGutters>
                <ListItemIcon><SubtitleIcon fontSize="small" /></ListItemIcon>
                <ListItemText
                  primary={c.label || c.name}
                  secondary={c.name}
                />
                <Button
                  size="small"
                  variant="outlined"
                  component="a"
                  href={c.url}
                  download={c.name}
                >
                  {i18n.t('downloadFile')}
                </Button>
              </ListItem>
            ))}
          </List>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>{i18n.t('close')}</Button>
      </DialogActions>
    </Dialog>
  )
}

export default DownloadDialog
