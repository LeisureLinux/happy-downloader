import { useEffect, useMemo, useState } from 'react'
import {
  Box,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography
} from '@mui/material'
import type { DLMetadata } from '../types'
import {
  availableSubtitleLangs,
  buildDownloadArgs,
  containers,
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
          onChange={(_, v) => v && set('media', v)}
        >
          <ToggleButton value="video" disabled={!canVideo}>
            视频+音频
          </ToggleButton>
          <ToggleButton value="audio" disabled={!canAudio}>
            仅音频
          </ToggleButton>
          <ToggleButton value="videoOnly" disabled={!canVideo}>
            仅视频
          </ToggleButton>
        </ToggleButtonGroup>

        <FormControl size="small" sx={{ minWidth: 160 }}>
          <InputLabel>分辨率上限</InputLabel>
          <Select
            label="分辨率上限"
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
          <InputLabel>容器格式</InputLabel>
          <Select
            label="容器格式"
            value={value.container}
            onChange={e => set('container', e.target.value)}
          >
            {containers.map(c => (
              <MenuItem key={c || 'auto'} value={c}>
                {c || '自动'}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      {/* ── 字幕 ───────────────────────────────────────────── */}
      <Stack direction="row" spacing={2} flexWrap="wrap" alignItems="center">
        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>字幕</InputLabel>
          <Select
            label="字幕"
            value={value.subtitleMode}
            onChange={e => set('subtitleMode', e.target.value as DownloadOptions['subtitleMode'])}
          >
            <MenuItem value="none">不下载</MenuItem>
            <MenuItem value="manual" disabled={manual.length === 0}>
              人工字幕{manual.length > 0 ? ` (${manual.length})` : ''}
            </MenuItem>
            <MenuItem value="auto" disabled={auto.length === 0}>
              自动生成{auto.length > 0 ? ` (${auto.length})` : ''}
            </MenuItem>
          </Select>
        </FormControl>

        {value.subtitleMode !== 'none' && (
          <>
            <FormControl size="small" sx={{ minWidth: 220 }}>
              <InputLabel>字幕语言</InputLabel>
              <Select
                multiple
                label="字幕语言"
                value={value.subtitleLangs}
                onChange={e => set('subtitleLangs', e.target.value as string[])}
                renderValue={v => (v as string[]).join(', ')}
              >
                {(offered.length > 0 ? offered : subtitleLangs).map(l => (
                  <MenuItem key={l} value={l}>
                    {l}
                    {manual.includes(l) && (
                      <Chip size="small" label="人工" sx={{ ml: 1, height: 18 }} />
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
              嵌入视频
            </ToggleButton>
          </>
        )}
      </Stack>

      {/* ── 实际参数 ───────────────────────────────────────── */}
      <Paper variant="outlined" sx={{ p: 1.5, bgcolor: 'action.hover' }}>
        <Typography variant="caption" color="text.secondary">
          将传给 yt-dlp 的参数
        </Typography>
        <Box
          component="pre"
          sx={{
            mt: 0.5, mb: 0, fontSize: '11.5px', fontFamily: 'Roboto Mono',
            whiteSpace: 'pre-wrap', wordBreak: 'break-all'
          }}
        >
          {renderArgs(args) || '(无附加参数)'}
        </Box>
      </Paper>
    </Stack>
  )
}

export default DownloadOptionsPanel
