import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  List,
  ListItem,
  ListItemText,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import SubtitleIcon from '@mui/icons-material/Subtitles'
import { useAtomValue } from 'jotai'
import { languageState, serverURL, youtubeCaptionLang } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'
import { CaptionTrack } from '../types'

// Captions for a file that was already downloaded.
//
// The video is fetched without captions, because YouTube rate-limits the
// caption endpoint far harder than it rate-limits video and a throttled
// caption request must not cost a video that already succeeded. So the
// captions come afterwards, on demand.
//
// Which languages are offered comes from the source. English and the
// interface's own language are pre-selected, because those are the two a
// person watching here is most likely to want; the rest are one click away.

type Props = {
  open: boolean
  path: string
  name: string
  onClose: () => void
  onDone: () => void
}

const CaptionsDialog: React.FC<Props> = ({ open, path, name, onClose, onDone }) => {
  const serverAddr = useAtomValue(serverURL)
  const language = useAtomValue(languageState)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [tracks, setTracks] = useState<CaptionTrack[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [filter, setFilter] = useState('')

  // Which caption languages the source has.
  //
  // The server asks yt-dlp, because the download record only remembers the
  // languages from the probe, and that is gone once the file has been on disk
  // for a while.
  const load = useCallback(async () => {
    setLoading(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/api/v1/captions?path=${encodeURIComponent(path)}`)

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const found: CaptionTrack[] = await res.json()

      // A popular video has 165 caption languages. Listing them all is not a
      // choice, it is a wall — so the ones most likely to be wanted come
      // first, and the rest are behind a filter.
      const preferred = ['en', youtubeCaptionLang[language]]
      const preferredSet = new Set(preferred)

      const sorted = [...found].sort((a, b) => {
        const ai = preferred.indexOf(a.language)
        const bi = preferred.indexOf(b.language)

        if (ai !== -1 || bi !== -1) {
          if (ai === -1) return 1
          if (bi === -1) return -1
          return ai - bi
        }

        // Authored captions before generated ones.
        if (a.automatic !== b.automatic) {
          return a.automatic ? 1 : -1
        }

        return (a.name || a.language).localeCompare(b.name || b.language)
      })

      setTracks(sorted)

      // English and the interface's own language, pre-selected.
      setSelected(new Set(found.filter(t => preferredSet.has(t.language)).map(t => t.language)))
    } catch (e) {
      // The server's message is plain English and says nothing about what to
      // do. A file with no remembered source cannot get captions, and the
      // useful thing to say is that, rather than "404".
      const message = String(e)

      setError(
        message.includes('no remembered source')
          ? i18n.t('captionsNoSource')
          : message
      )

      setTracks([])
    } finally {
      setLoading(false)
    }
  }, [serverAddr, path, language])

  useEffect(() => {
    if (open) {
      void load()
    }
  }, [open, load])

  // The filter matches both the language code and the name, so "zh" finds
  // Chinese and "auto" finds the generated ones.
  const filtered = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle) return tracks

    return tracks.filter(t =>
      t.language.toLowerCase().includes(needle) ||
      (t.name ?? '').toLowerCase().includes(needle)
    )
  }, [tracks, filter])

  const toggle = (code: string) => setSelected(prev => {
    const next = new Set(prev)
    next.has(code) ? next.delete(code) : next.add(code)
    return next
  })

  const download = async () => {
    if (selected.size === 0) return

    setBusy(true)

    try {
      const res = await fetch(`${serverAddr}/api/v1/captions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, languages: [...selected] })
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const result = await res.json()

      const written = result.written?.length ?? 0
      const skipped = Object.keys(result.skipped ?? {}).length

      pushMessage(
        written > 0
          ? i18n.t('captionsDownloaded').replace('{count}', String(written))
          : i18n.t('captionsNone'),
        written > 0 ? 'success' : 'info'
      )

      if (skipped > 0) {
        pushMessage(i18n.t('captionsSkipped').replace('{count}', String(skipped)), 'info')
      }

      onDone()
      onClose()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Stack direction="row" spacing={1} alignItems="center">
          <SubtitleIcon />
          <span>{i18n.t('captionsFor')}</span>
        </Stack>
        <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
          {name}
        </Typography>
      </DialogTitle>

      <DialogContent>
        {loading && (
          <Stack direction="row" spacing={1} alignItems="center" sx={{ py: 2 }}>
            <CircularProgress size={16} />
            <span>{i18n.t('captionsLoading')}</span>
          </Stack>
        )}

        {error && <Alert severity="error">{error}</Alert>}

        {!loading && !error && tracks.length === 0 && (
          <Alert severity="info">{i18n.t('captionsUnavailable')}</Alert>
        )}

        {!loading && tracks.length > 0 && (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              {i18n.t('captionsHint')}
            </Typography>

            <TextField
              size="small"
              fullWidth
              placeholder={i18n.t('captionsFilter')}
              value={filter}
              onChange={e => setFilter(e.target.value)}
              sx={{ mb: 1 }}
            />

            <List dense disablePadding sx={{ maxHeight: 320, overflow: 'auto' }}>
              {filtered.map(t => (
                <ListItem key={`${t.language}-${t.automatic}`} disableGutters>
                  <FormControlLabel
                    control={
                      <Checkbox
                        size="small"
                        checked={selected.has(t.language)}
                        onChange={() => toggle(t.language)}
                      />
                    }
                    label={
                      <ListItemText
                        // The name is what the source calls the track, e.g.
                        // "Chinese (Simplified)". The code is only added when
                        // the name does not already contain it, because
                        // "Chinese (Simplified) (zh-Hans)" is noise.
                        primary={
                          t.name && t.name !== t.language && !t.name.includes(t.language)
                            ? `${t.name} (${t.language})`
                            : (t.name || t.language)
                        }
                        secondary={t.automatic ? i18n.t('captionsAutomatic') : undefined}
                      />
                    }
                  />
                </ListItem>
              ))}
            </List>
          </>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>{i18n.t('cancel')}</Button>
        <Button
          variant="contained"
          disabled={selected.size === 0 || busy}
          onClick={download}
        >
          {busy ? i18n.t('captionsDownloading') : i18n.t('captionsDownload')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default CaptionsDialog
