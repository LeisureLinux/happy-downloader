import { useEffect, useMemo, useState } from 'react'
import {
  Box,
  Chip,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Switch,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography
} from '@mui/material'
import DevicesIcon from '@mui/icons-material/Devices'
import type { DLMetadata } from '../types'
import { useI18n } from '../hooks/useI18n'
import {
  availableSubtitleLangs,
  buildDownloadArgs,
  containersFor,
  defaultOptions,
  hasAudioStream,
  hasVideoStream,
  heightPresets,
  renderArgs,
  subtitleLangs,
  type DownloadOptions
} from '../lib/downloadOptions'

type Props = {
  metadata?: DLMetadata
  value: DownloadOptions
  onChange: (o: DownloadOptions) => void
}

// The choices that shape the download, with the command line they produce
// rendered underneath.
//
// The preview is the point: every option here changes which flags yt-dlp
// receives, and reading the resolved line is the only way to know what was
// actually asked for. It is the same string the downloader will run.
const DownloadOptionsPanel: React.FC<Props> = ({ metadata, value, onChange }) => {
  const { i18n } = useI18n()

  const set = <K extends keyof DownloadOptions>(k: K, v: DownloadOptions[K]) =>
    onChange({ ...value, [k]: v })

  const { manual, auto } = useMemo(() => availableSubtitleLangs(metadata), [metadata])

  // Languages the video actually offers, falling back to the common list so
  // the control is usable before metadata arrives.
  const offered = useMemo(() => {
    const seen = new Set<string>()
    return [...manual, ...auto].filter(l => (seen.has(l) ? false : (seen.add(l), true)))
  }, [manual, auto])

  const args = useMemo(() => buildDownloadArgs(value), [value])

  const canAudio = hasAudioStream(metadata)
  const canVideo = hasVideoStream(metadata)

  return (
    <Stack spacing={2}>
      {/* ── 要什么 ─────────────────────────────────────────── */}
      <Stack direction="row" spacing={2} flexWrap="wrap" alignItems="center">
        <ToggleButtonGroup
          exclusive
          size="small"
          value={value.media}
          onChange={(_, v) => {
            if (!v) return
            // Switching to audio leaves "mp4" selected, which yt-dlp cannot
            // produce from an audio-only source. Reset rather than silently
            // passing an impossible target.
            const allowed = containersFor(v as DownloadOptions['media'])
            onChange({
              ...value,
              media: v as DownloadOptions['media'],
              container: allowed.includes(value.container) ? value.container : '',
            })
          }}
        >
          <ToggleButton value="video" disabled={!canVideo}>
            {i18n.t('mediaVideoAudio')}
          </ToggleButton>
          <ToggleButton value="audio" disabled={!canAudio}>
            {i18n.t('mediaAudioOnly')}
          </ToggleButton>
          <ToggleButton value="videoOnly" disabled={!canVideo}>
            {i18n.t('mediaVideoOnly')}
          </ToggleButton>
        </ToggleButtonGroup>

        <FormControl size="small" sx={{ minWidth: 160 }}>
          <InputLabel>{i18n.t('maxHeightLabel')}</InputLabel>
          <Select
            label={i18n.t('maxHeightLabel')}
            value={String(value.maxHeight)}
            onChange={e => set('maxHeight', Number(e.target.value))}
          >
            {heightPresets.map(p => (
              <MenuItem key={p.value} value={String(p.value)}>
                {p.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 140 }}>
          <InputLabel>{value.media === 'audio' ? i18n.t('audioFormatLabel') : i18n.t('containerFormatLabel')}</InputLabel>
          <Select
            label={value.media === 'audio' ? i18n.t('audioFormatLabel') : i18n.t('containerFormatLabel')}
            value={value.container}
            onChange={e => set('container', e.target.value)}
          >
            {containersFor(value.media).map(c => (
              <MenuItem key={c || 'auto'} value={c}>
                {c || i18n.t('autoOption')}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      {/* ── 兼容模式 ───────────────────────────────────────── */}
      <Stack direction="row" spacing={1} alignItems="center">
        <FormControlLabel
          control={
            <Switch
              size="small"
              checked={value.compatible}
              onChange={e => set('compatible', e.target.checked)}
            />
          }
          label={
            <Stack direction="row" spacing={0.5} alignItems="center">
              <DevicesIcon fontSize="small" />
              <span>{i18n.t('compatibleMode')}</span>
            </Stack>
          }
        />
        {/* 代价要说清楚：H.264 的码率 YouTube 给得比 AV1 高，文件可能大很多。 */}
        <Typography variant="caption" color="text.secondary">
          {i18n.t('compatibleModeHint')}
        </Typography>
      </Stack>

      {/* ── 字幕 ───────────────────────────────────────────── */}
      <Stack direction="row" spacing={2} flexWrap="wrap" alignItems="center">
        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>{i18n.t('subtitlesLabel')}</InputLabel>
          <Select
            label={i18n.t('subtitlesLabel')}
            value={value.subtitleMode}
            onChange={e => set('subtitleMode', e.target.value as DownloadOptions['subtitleMode'])}
          >
            <MenuItem value="none">{i18n.t('subtitlesNone')}</MenuItem>
            <MenuItem value="manual" disabled={manual.length === 0}>
              {i18n.t('subtitlesManual')}{manual.length > 0 ? ` (${manual.length})` : ''}
            </MenuItem>
            <MenuItem value="auto" disabled={auto.length === 0}>
              {i18n.t('subtitlesAuto')}{auto.length > 0 ? ` (${auto.length})` : ''}
            </MenuItem>
          </Select>
        </FormControl>

        {value.subtitleMode !== 'none' && (
          <>
            <FormControl size="small" sx={{ minWidth: 220 }}>
              <InputLabel>{i18n.t('subtitleLangsLabel')}</InputLabel>
              <Select
                multiple
                label={i18n.t('subtitleLangsLabel')}
                value={value.subtitleLangs}
                onChange={e => set('subtitleLangs', e.target.value as string[])}
                renderValue={v => (v as string[]).join(', ')}
              >
                {(offered.length > 0 ? offered : subtitleLangs).map(l => (
                  <MenuItem key={l} value={l}>
                    {l}
                    {manual.includes(l) && (
                      <Chip size="small" label={i18n.t('manualBadge')} sx={{ ml: 1, height: 18 }} />
                    )}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <ToggleButton
              size="small"
              value="embed"
              selected={value.embedSubtitles}
              onChange={() => set('embedSubtitles', !value.embedSubtitles)}
            >
              {i18n.t('embedSubtitlesLabel')}
            </ToggleButton>
          </>
        )}
      </Stack>

      {/* ── 实际参数 ───────────────────────────────────────── */}
      <Paper variant="outlined" sx={{ p: 1.5, bgcolor: 'action.hover' }}>
        <Typography variant="caption" color="text.secondary">
          {i18n.t('argsPreviewLabel')}
        </Typography>
        <Box
          component="pre"
          sx={{
            mt: 0.5, mb: 0, fontSize: '11.5px', fontFamily: 'Roboto Mono',
            whiteSpace: 'pre-wrap', wordBreak: 'break-all'
          }}
        >
          {renderArgs(args) || i18n.t('argsPreviewEmpty')}
        </Box>
      </Paper>
    </Stack>
  )
}

export default DownloadOptionsPanel
