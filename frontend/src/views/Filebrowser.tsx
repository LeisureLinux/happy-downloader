import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Box,
  Button,
  ButtonGroup,
  Checkbox,
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
import FolderIcon from '@mui/icons-material/Folder'
import { matchW } from 'fp-ts/lib/TaskEither'
import { pipe } from 'fp-ts/lib/function'
import { serverURL } from '../atoms/settings'
import { useToast } from '../hooks/toast'
import { useI18n } from '../hooks/useI18n'
import { ffetch } from '../lib/httpClient'
import { DirectoryEntry } from '../types'
import { base64URLEncode, formatSize } from '../utils'
import { useAtomValue } from 'jotai'

// A row is a file or a folder. Selection is by path, which is unique within a
// listing, unlike the name the previous implementation keyed on.
type Row = DirectoryEntry & { selected: boolean }

const fileAction = (serverAddr: string, path: string, kind: 'view' | 'download') => {
  const encoded = base64URLEncode(path)
  window.open(`${serverAddr}/filebrowser/${kind === 'view' ? 'v' : 'd'}/${encoded}?token=${localStorage.getItem('token')}`)
}

export default function Filebrowser() {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [rows, setRows] = useState<Row[]>([])
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
        body: JSON.stringify({ subdir: dir })
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
          <Typography variant="h6">{i18n.t('browseButtonLabel')}</Typography>
          <Box flexGrow={1} />
          {/* 批量操作：只对选中的文件生效 */}
          <ButtonGroup size="small">
            <Tooltip title={i18n.t('playSelected')}>
              <span>
                <IconButton
                  size="small"
                  disabled={selectedFiles.length === 0}
                  onClick={() => selectedFiles.forEach(f => fileAction(serverAddr, f.path, 'view'))}
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
                  onClick={() => selectedFiles.forEach(f => fileAction(serverAddr, f.path, 'download'))}
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
                  <TableCell>{i18n.t('colFileName')}</TableCell>
                  <TableCell align="right">{i18n.t('colDownloadTime')}</TableCell>
                  <TableCell align="right">{i18n.t('colFileSize')}</TableCell>
                  <TableCell align="right">{i18n.t('colActions')}</TableCell>
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
                    <TableCell colSpan={3} />
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
                    <TableCell>{f.name}</TableCell>
                    <TableCell align="right">
                      {f.modTime ? new Date(f.modTime).toLocaleString() : '—'}
                    </TableCell>
                    <TableCell align="right">{formatSize(f.size)}</TableCell>
                    <TableCell align="right">
                      {/* 与下载历史用同一组图标按钮：同一类操作在两处长得一样 */}
                      <ButtonGroup size="small">
                        <Tooltip title={i18n.t('playSelected')}>
                          <IconButton size="small" onClick={() => fileAction(serverAddr, f.path, 'view')}>
                            <SmartDisplayIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title={i18n.t('downloadSelected')}>
                          <IconButton size="small" onClick={() => fileAction(serverAddr, f.path, 'download')}>
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
    </Container>
  )
}
