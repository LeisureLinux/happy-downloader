import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Box,
  Button,
  ButtonGroup,
  Checkbox,
  Chip,
  CircularProgress,
  Container,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography
} from '@mui/material'
import DeleteIcon from '@mui/icons-material/Delete'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import SmartDisplayIcon from '@mui/icons-material/SmartDisplay'
import SubtitleIcon from '@mui/icons-material/Subtitles'
import FolderIcon from '@mui/icons-material/Folder'
import { matchW } from 'fp-ts/lib/TaskEither'
import { pipe } from 'fp-ts/lib/function'
import { serverURL } from '../atoms/settings'
import { useToast } from '../hooks/toast'
import { useI18n } from '../hooks/useI18n'
import { ffetch } from '../lib/httpClient'
import { DirectoryEntry, ProcessStatus } from '../types'
import { base64URLEncode, formatDuration, formatSize } from '../utils'
import { useAtomValue } from 'jotai'
import { activeDownloadsState } from '../atoms/downloads'
import Player from '../components/Player'
import CaptionsDialog from '../components/CaptionsDialog'

// A row is a file or a folder. Selection is by path, which is unique within a
// listing, unlike the name the previous implementation keyed on.
type Row = DirectoryEntry & { selected: boolean }

// Downloads open in a new tab. Playback does not: the browser's own player
// cannot offer a caption track, and choosing one is the reason the captions
// were downloaded.
const downloadFile = (serverAddr: string, path: string) => {
  const encoded = base64URLEncode(path)
  window.open(`${serverAddr}/filebrowser/d/${encoded}`)
}

export default function Filebrowser() {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [rows, setRows] = useState<Row[]>([])
  // The file being played, if any. The player is a dialog over the list, so
  // the list stays where it was.
  const [playing, setPlaying] = useState<Row | null>(null)
  // The file whose captions are being fetched.
  const [captioning, setCaptioning] = useState<Row | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [subdir, setSubdir] = useState('')
  const [loading, setLoading] = useState(true)

  // useToast() returns a fresh object every render, so pushMessage is a new
  // function reference each time. Putting it in a dependency array recreates
  // list() on every render and the effect below re-fires in a loop — which is
  // exactly what happened here: thousands of requests before the page settled.
  const notifyRef = useRef(pushMessage)
  notifyRef.current = pushMessage

  const list = useCallback(async (dir: string) => {
    setLoading(true)
    try {
      const res = await fetch(`${serverAddr}/filebrowser/downloaded`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        // Newest first. The server sorts by modification time, which for a
        // finished download is when yt-dlp wrote the file — close enough to
        // when it completed to be what the user means by "downloaded at".
        body: JSON.stringify({ subdir: dir, orderBy: 'modtime' })
      })
      if (!res.ok) throw new Error(await res.text())
      const data: DirectoryEntry[] = await res.json()
      setRows(data.map(d => ({ ...d, selected: false })))
      setSelected(new Set())
    } catch (e) {
      notifyRef.current(String(e), 'error')
    } finally {
      setLoading(false)
    }
  }, [serverAddr])

  useEffect(() => { void list(subdir) }, [list, subdir])

  // A finished download is a file that belongs here, so the list should gain it
  // without a manual refresh. Watching the completed count rather than the whole
  // download state keeps progress ticks from re-listing the directory on every
  // percentage point.
  const completedCount = useAtomValue(activeDownloadsState)
    .filter(d => d.progress.process_status === ProcessStatus.COMPLETED).length

  const seenCompleted = useRef<number | null>(null)
  useEffect(() => {
    if (seenCompleted.current === null) {
      seenCompleted.current = completedCount
      return
    }
    if (completedCount !== seenCompleted.current) {
      seenCompleted.current = completedCount
      void list(subdir)
    }
  }, [completedCount, list, subdir])

  const files = useMemo(() => rows.filter(r => !r.isDirectory), [rows])
  const folders = useMemo(() => rows.filter(r => r.isDirectory), [rows])

  const allSelected = files.length > 0 && files.every(f => selected.has(f.path))

  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(files.map(f => f.path)))
  }

  const toggleOne = (path: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      next.has(path) ? next.delete(path) : next.add(path)
      return next
    })
  }

  const remove = async (paths: string[]) => {
    if (paths.length === 0) return
    if (!window.confirm(i18n.t('confirmDeleteFiles'))) return

    for (const p of paths) {
      try {
        await fetch(`${serverAddr}/filebrowser/delete`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ path: p })
        })
      } catch (e) {
        notifyRef.current(String(e), 'error')
      }
    }
    await list(subdir)
  }

  const selectedFiles = files.filter(f => selected.has(f.path))

  return (
    <Container maxWidth="xl" sx={{ mt: 2, mb: 8 }}>
      <Paper sx={{ p: 2 }}>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
          <Typography variant="h6" color="primary">{i18n.t('browseButtonLabel')}</Typography>
          <Box flexGrow={1} />
          {/* 批量操作：只对选中的文件生效 */}
          <ButtonGroup size="small">
            <Tooltip title={i18n.t('playSelected')}>
              <span>
                <IconButton
                  size="small"
                  disabled={selectedFiles.length === 0}
                  onClick={() => setPlaying(selectedFiles[0] ?? null)}
                >
                  <SmartDisplayIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title={i18n.t('downloadSelected')}>
              <span>
                <IconButton
                  size="small"
                  disabled={selectedFiles.length === 0}
                  onClick={() => selectedFiles.forEach(f => downloadFile(serverAddr, f.path))}
                >
                  <FileDownloadIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title={i18n.t('deleteSelected')}>
              <span>
                <IconButton
                  size="small"
                  color="error"
                  disabled={selectedFiles.length === 0}
                  onClick={() => remove(selectedFiles.map(f => f.path))}
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
          </ButtonGroup>
        </Stack>

        {subdir !== '' && (
          <Button size="small" onClick={() => setSubdir('')} sx={{ mb: 1 }}>
            ← {i18n.t('goUp')}
          </Button>
        )}

        {loading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
            <CircularProgress size={24} />
          </Box>
        ) : rows.length === 0 ? (
          <Typography color="text.secondary">{i18n.t('noFilesFound')}</Typography>
        ) : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox">
                    <Checkbox
                      checked={allSelected}
                      indeterminate={!allSelected && selected.size > 0}
                      onChange={toggleAll}
                      inputProps={{ 'aria-label': 'select all' }}
                    />
                  </TableCell>
                  {/* 表头用主题色：这些是列名，和标题一样是该被强调的东西，
                      默认灰色让表格和页面其他部分的配色脱节。 */}
                  <TableCell sx={{ color: 'primary.main' }}>{i18n.t('colFileName')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main' }}>{i18n.t('colDuration')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main' }}>{i18n.t('colDownloadTime')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main' }}>{i18n.t('colFileSize')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main' }}>{i18n.t('colActions')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {folders.map(f => (
                  <TableRow key={f.path} hover>
                    <TableCell padding="checkbox" />
                    <TableCell>
                      <Stack direction="row" spacing={1} alignItems="center">
                        <FolderIcon fontSize="small" />
                        <Button size="small" onClick={() => setSubdir(f.path)}>
                          {f.name}
                        </Button>
                      </Stack>
                    </TableCell>
                    <TableCell colSpan={4} />
                  </TableRow>
                ))}
                {files.map(f => (
                  <TableRow key={f.path} hover selected={selected.has(f.path)}>
                    <TableCell padding="checkbox">
                      <Checkbox
                        checked={selected.has(f.path)}
                        onChange={() => toggleOne(f.path)}
                      />
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={1} alignItems="center">
                        <span>{f.name}</span>
                        {/* 字幕文件的数量：一眼看出这个视频带了字幕 */}
                        {f.subtitles && f.subtitles.length > 0 && (
                          <Chip
                            size="small"
                            variant="outlined"
                            label={`${f.subtitles.length} ${i18n.t('subtitleCount')}`}
                            sx={{ height: 20, fontSize: 11 }}
                          />
                        )}
                      </Stack>
                    </TableCell>
                    <TableCell align="right">{formatDuration(f.duration) || '—'}</TableCell>
                    <TableCell align="right">
                      {f.modTime ? new Date(f.modTime).toLocaleString() : '—'}
                    </TableCell>
                    <TableCell align="right">{formatSize(f.size)}</TableCell>
                    <TableCell align="right">
                      {/* 与下载历史用同一组图标按钮：同一类操作在两处长得一样 */}
                      <ButtonGroup size="small">
                        <Tooltip title={i18n.t('playSelected')}>
                          <IconButton size="small" onClick={() => setPlaying(f)}>
                            <SmartDisplayIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        {/* 字幕单独下载：YouTube 限流字幕端点比视频严格得多，
                            把它和视频绑在一起会让限流毁掉整个下载。 */}
                        {f.isVideo && (
                          <Tooltip title={i18n.t('captionsButton')}>
                            <IconButton size="small" onClick={() => setCaptioning(f)}>
                              <SubtitleIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        )}
                        <Tooltip title={i18n.t('downloadSelected')}>
                          <IconButton size="small" onClick={() => downloadFile(serverAddr, f.path)}>
                            <FileDownloadIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title={i18n.t('deleteSelected')}>
                          <IconButton size="small" onClick={() => remove([f.path])}>
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </ButtonGroup>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>

      <CaptionsDialog
        open={captioning !== null}
        path={captioning?.path ?? ''}
        name={captioning?.name ?? ''}
        onClose={() => setCaptioning(null)}
        onDone={() => { void list(subdir) }}
      />

      <Player
        open={playing !== null}
        serverAddr={serverAddr}
        path={playing?.path ?? ''}
        name={playing?.name ?? ''}
        subtitles={playing?.subtitles}
        onClose={() => setPlaying(null)}
      />
    </Container>
  )
}
