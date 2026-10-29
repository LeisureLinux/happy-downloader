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
import { isLivestreamURL } from '../lib/livestream'
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
// How long a livestream should be recorded, in minutes.
//
// A livestream does not end, so "download it" is a question without an
// answer unless a length is given: the recording went on until somebody
// noticed. Zero means until it is stopped by hand.
const recordMinutesAtom = atom(60)
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
  const [recordMinutes, setRecordMinutes] = useAtom(recordMinutesAtom)
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

    // The list, not a joined string: the client used to re-parse a string and
    // an argument containing "-o" was cut in half.
    const finalArgs = [
      ...buildDownloadArgs(options),
      ...customArgs.split(/\s+/).filter(Boolean),
    ]

    try {
      // A livestream does not end, so it does not go down the ordinary path.
      //
      // The ordinary download waits for the file to be finished, and a
      // livestream is never finished: the download sat in the queue at
      // "preparing" forever, with no file and nothing to say what it was
      // doing. The monitor waits for the stream to start instead, and it can
      // be stopped.
      if (isLivestreamURL(url)) {
        await client.execLivestream(url.trim(), recordMinutes)
        pushMessage(`${i18n.t('livestreamQueued')}: ${url.trim()}`, 'success')
        setUrl('')
        return
      }

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
        <Typography variant="h6" color="primary">{i18n.t('downloadsButtonLabel')}</Typography>

        {/* URL 输入框和"选择格式"按钮，居中，宽度是固定的。
            不是百分比：七成的宽度在电脑上很好，在手机上就是屏幕的
            七成，再减去按钮，输入框只剩一条缝。
            固定宽度在两端都好看 —— 电脑上是一块 Google 那样居中的
            搜索框。 */}
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={1}
          alignItems="flex-start"
          sx={{
            width: '100%',
            maxWidth: 620,
            // 居中靠的是 alignSelf，不是 margin: auto。
            // 外层的 Stack 是 flex 列布局，align-items 默认是 stretch，
            // 子项的宽度因此是确定的 —— 自动外边距在这种情形下不
            // 参与计算，于是这一行贴着左边，而不是在页面中间。
            alignSelf: 'center',
            mx: 'auto',
          }}
        >
          <TextField
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
            // The field gives up its width to the button rather than the
            // other way round: "选择格式" wrapped onto two lines, and a URL
            // box that is a little shorter is not a problem.
            //
            // On a phone the button is below, so the field takes the whole
            // row. Without width: 100% a multiline field in a column flex
            // container shrinks to what its content needs, which on a phone
            // was a box 181 pixels wide.
            sx={{ flexGrow: 1, width: { xs: '100%', sm: 'auto' }, minWidth: 0 }}
          />
          <Button
            variant="contained"
            disabled={!url.trim() || probing}
            onClick={() => void probe()}
            sx={{
              flexShrink: 0,
              whiteSpace: 'nowrap',
              // Wide enough for the longest label — "Reading metadata 12s" — so
              // the row does not jump while the probe runs. The old label was
              // "Reading...", which said what the spinner said and nothing more.
              minWidth: 186,
              // On a phone the button is on its own line below the field, so
              // it takes the width of the row. The field needs every pixel
              // it can get: it was down to a slit.
              width: { xs: '100%', sm: 'auto' },
              // Probing disables the button so a second request cannot start,
              // but the probe is the thing happening on the page: MUI's grey
              // disabled state made the wait look like an idle button.
              '&.Mui-disabled': probing
                ? {
                    backgroundColor: 'primary.main',
                    color: 'primary.contrastText',
                    opacity: 0.75,
                  }
                : {},
            }}
          >
            {probing
              ? `${i18n.t('probing')} ${elapsed}s`
              : i18n.t('selectFormatButton')}
          </Button>
        </Stack>

        <FormControlLabel
          control={
            <Checkbox checked={asPlaylist} onChange={() => setAsPlaylist(v => !v)} />
          }
          label={i18n.t('playlistCheckbox')}
        />

        {/* 直播要录多久。直播不会结束，所以不问清楚就会一直录下去，
            直到有人想起来。 */}
        {isLivestreamURL(url) && (
          <TextField
            size="small"
            type="number"
            label={i18n.t('recordMinutesLabel')}
            value={recordMinutes}
            onChange={e => setRecordMinutes(Math.max(0, Number(e.target.value) || 0))}
            inputProps={{ min: 0, style: { width: 90 } }}
            helperText={i18n.t('recordMinutesHint')}
            sx={{ maxWidth: 260 }}
          />
        )}

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
