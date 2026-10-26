import { useEffect, useRef, useState } from 'react'
import {
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  Paper,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import DownloadOptionsPanel from '../components/DownloadOptionsPanel'
import { useRPC } from '../hooks/useRPC'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'
import { settingsState } from '../atoms/settings'
import { customArgsState } from '../atoms/downloadTemplate'
import {
  buildDownloadArgs,
  defaultOptions,
  renderArgs,
  type DownloadOptions
} from '../lib/downloadOptions'
import type { DLMetadata } from '../types'
import { atom, useAtom, useAtomValue } from 'jotai'

// The form's state lives outside the component. Navigating to the log page and
// back used to unmount this component and discard the URL, the metadata and
// every option chosen — and the metadata takes twenty seconds to arrive, so
// looking at the log while waiting is the obvious thing to do, and it lost the
// work. Module atoms survive the unmount.
const urlAtom = atom('')
const metaAtom = atom<DLMetadata | undefined>(undefined)
const optionsAtom = atom<DownloadOptions>(defaultOptions)
const playlistAtom = atom(false)
const filenameAtom = atom('')
// Deliberately not persisted: a probe in flight belongs to this tab, and
// restoring a stuck spinner after a reload would be worse than starting over.
const probingAtom = atom(false)

// The download flow, inline on the page: paste a URL, see what the source
// actually offers, choose what to keep, download.
//
// This replaces a dialog behind a floating action button. The steps are the
// same — probe the URL, show metadata, confirm options — but they happen on
// the page, so the URL and the resulting choices stay visible together and
// there is no modal to dismiss to get back to the list.
const DownloadForm: React.FC = () => {
  const { i18n } = useI18n()
  const { client } = useRPC()
  const { pushMessage } = useToast()
  const settings = useAtomValue(settingsState)
  const customArgs = useAtomValue(customArgsState)

  const [url, setUrl] = useAtom(urlAtom)
  const [meta, setMeta] = useAtom(metaAtom)
  const [options, setOptions] = useAtom(optionsAtom)
  const [probing, setProbing] = useAtom(probingAtom)
  const [asPlaylist, setAsPlaylist] = useAtom(playlistAtom)
  const [filename, setFilename] = useAtom(filenameAtom)

  // Elapsed seconds while probing. Twenty seconds of a disabled button and no
  // other change reads as a hang, and the fix is to show that time is passing.
  const [elapsed, setElapsed] = useState(0)
  const startedAt = useRef(0)

  useEffect(() => {
    if (!probing) {
      setElapsed(0)
      return
    }
    startedAt.current = Date.now()
    const t = setInterval(
      () => setElapsed(Math.floor((Date.now() - startedAt.current) / 1000)),
      1000
    )
    return () => clearInterval(t)
  }, [probing])

  const probe = async () => {
    const trimmed = url.trim()
    if (!trimmed) return

    setProbing(true)
    try {
      const res = await client.formats(trimmed)
      const result = res?.result
      if (!result) throw new Error('no metadata returned')

      if (result._type === 'playlist') {
        pushMessage(i18n.t('playlistDetected'), 'info')
      }

      setMeta(result)
    } catch (e) {
      pushMessage(String(e), 'error')
    } finally {
      setProbing(false)
    }
  }

  const start = async () => {
    if (!url.trim()) return

    const args = renderArgs(buildDownloadArgs(options))
    const finalArgs = `${args} ${customArgs}`.replace(/  +/g, ' ').trim()

    try {
      await client.download({
        url: url.trim(),
        args: finalArgs,
        pathOverride: '',
        renameTo: settings.fileRenaming ? filename : '',
        playlist: asPlaylist,
      })
      pushMessage(`${i18n.t('downloadQueued')}: ${url.trim()}`, 'success')
      setUrl('')
      setMeta(undefined)
      setOptions(defaultOptions)
    } catch (e) {
      pushMessage(String(e), 'error')
    }
  }

  const reset = () => {
    setMeta(undefined)
    setOptions(defaultOptions)
  }

  return (
    <Paper sx={{ p: 2, mb: 3 }}>
      <Stack spacing={2}>
        <Typography variant="h6">{i18n.t('downloadsButtonLabel')}</Typography>

        <Stack direction="row" spacing={1} alignItems="flex-start">
          <TextField
            fullWidth
            multiline
            minRows={1}
            maxRows={6}
            label={i18n.t('urlInput')}
            value={url}
            onChange={e => setUrl(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                void probe()
              }
            }}
          />
          <Button
            variant="contained"
            disabled={!url.trim() || probing}
            onClick={() => void probe()}
          >
            {probing ? `${i18n.t('probing')} ${elapsed}s` : i18n.t('selectFormatButton')}
          </Button>
        </Stack>

        <FormControlLabel
          control={
            <Checkbox checked={asPlaylist} onChange={() => setAsPlaylist(v => !v)} />
          }
          label={i18n.t('playlistCheckbox')}
        />

        {meta && (
          <Box>
            <Stack direction="row" spacing={2} sx={{ mb: 2 }}>
              {meta.thumbnail && (
                <img
                  src={meta.thumbnail}
                  height={140}
                  style={{ objectFit: 'cover', borderRadius: 4 }}
                  alt=""
                />
              )}
              <Box>
                <Typography variant="subtitle1">{meta.title}</Typography>
                {!!meta.duration && (
                  <Typography variant="caption" color="text.secondary">
                    {Math.round(meta.duration / 60)} min
                  </Typography>
                )}
              </Box>
            </Stack>

            <DownloadOptionsPanel
              metadata={meta}
              value={options}
              onChange={setOptions}
            />

            {settings.fileRenaming && (
              <TextField
                fullWidth
                sx={{ mt: 2 }}
                label={i18n.t('customFilename')}
                value={filename}
                onChange={e => setFilename(e.target.value)}
              />
            )}

            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button variant="contained" onClick={() => void start()}>
                {i18n.t('startButton')}
              </Button>
              <Button onClick={reset}>{i18n.t('clearCompletedButton')}</Button>
            </Stack>
          </Box>
        )}
      </Stack>
    </Paper>
  )
}

export default DownloadForm
