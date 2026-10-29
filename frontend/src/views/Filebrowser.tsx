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
import CreateNewFolderIcon from '@mui/icons-material/CreateNewFolder'
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMove'
import DriveFileRenameOutlineIcon from '@mui/icons-material/DriveFileRenameOutline'
import { matchW } from 'fp-ts/lib/TaskEither'
import { pipe } from 'fp-ts/lib/function'
import { serverURL } from '../atoms/settings'
import { useToast } from '../hooks/toast'
import { useRPC } from '../hooks/useRPC'
import { useI18n } from '../hooks/useI18n'
import { ffetch } from '../lib/httpClient'
import { DirectoryEntry, ProcessStatus } from '../types'
import { base64URLEncode, formatDuration, formatSize } from '../utils'
import { useAtomValue } from 'jotai'
import { useTheme } from '@mui/material/styles'
import { activeDownloadsState } from '../atoms/downloads'
import { captionDoneCountState, captionJobsState } from '../atoms/captions'
import Player from '../components/Player'
import CaptionsDialog from '../components/CaptionsDialog'
import DownloadDialog from '../components/DownloadDialog'
import FileActions from '../components/FileActions'
import FolderDialog from '../components/FolderDialog'
import { isDark, siteIcon } from '../components/SiteIcon'

// A row is a file or a folder. Selection is by path, which is unique within a
// listing, unlike the name the previous implementation keyed on.
type Row = DirectoryEntry & { selected: boolean }

// The last part of a path: the name of the directory it names.
const baseName = (path: string): string => {
  const cut = path.lastIndexOf('/')
  if (cut < 0) return path

  return path.slice(cut + 1)
}

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
  const { client } = useRPC()
  // The accent colour, for a site that has no colour of its own.
  const theme = useTheme()

  const [rows, setRows] = useState<Row[]>([])
  // The file being played, if any. The player is a dialog over the list, so
  // the list stays where it was.
  const [playing, setPlaying] = useState<Row | null>(null)
  // The file whose captions are being fetched.
  const [captioning, setCaptioning] = useState<Row | null>(null)
  // The file whose download options are being shown.
  const [downloading, setDownloading] = useState<Row | null>(null)
  // The folder dialog: making a folder, moving files into one, or renaming one.
  const [folderDialog, setFolderDialog] = useState<'create' | 'move' | 'rename' | null>(null)
  // The folder being renamed.
  const [renaming, setRenaming] = useState<Row | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [subdir, setSubdir] = useState('')
  // The directory the "up" button goes to. It comes from the server, which is
  // the only thing that knows where the download directory is: the parent of a
  // first-level folder is the download directory, and that cannot be worked
  // out from the path alone.
  const [parent, setParent] = useState('')
  // The directory being listed, and its name. The name is what says where
  // you are: inside a folder, the list looks the same as the list outside it,
  // and without this there is nothing to say so.
  const [subdirPath, setSubdirPath] = useState('')
  const [subdirName, setSubdirName] = useState('')
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
      const data = await res.json()

      // The answer is an object, because the interface needs to know where
      // the root is: the "up" button goes to the parent directory, and the
      // parent of a first-level folder is the download directory — which
      // cannot be worked out from the path alone.
      const listing = data as {
        files?: DirectoryEntry[]
        parent?: string
        subDir?: string
      }

      // An empty folder used to come back as null, and .map() on null is a
      // crash that took the whole page with it.
      setRows((listing.files ?? []).map(d => ({ ...d, selected: false })))
      setParent(listing.parent ?? '')

      // The name of the directory being listed — the last part of its path.
      setSubdirPath(listing.subDir ?? '')
      setSubdirName(baseName(listing.subDir ?? ''))
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

  // The caption downloads that are running, by file path. The row says what is
  // happening to it while the server works, which is the only place the wait
  // is visible: the dialog closed as soon as the request was accepted.
  const captionJobs = useAtomValue(captionJobsState)

  // The caption files land on disk when the job finishes, so the directory has
  // to be listed again — otherwise the new subtitle files stay invisible until
  // the page is reloaded.
  const captionDoneCount = useAtomValue(captionDoneCountState)
  const seenCaptionDone = useRef<number | null>(null)

  useEffect(() => {
    if (seenCaptionDone.current === null) {
      seenCaptionDone.current = captionDoneCount
      return
    }
    if (captionDoneCount !== seenCaptionDone.current) {
      seenCaptionDone.current = captionDoneCount
      void list(subdir)
    }
  }, [captionDoneCount, list, subdir])

  const files = useMemo(() => rows.filter(r => !r.isDirectory), [rows])
  const folders = useMemo(() => rows.filter(r => r.isDirectory), [rows])

  // Where a file can be moved to: every folder under the download directory,
  // not only the ones in the folder being looked at.
  //
  // The list used to be the subfolders of the current directory, which meant a
  // file could be moved down but never up — moving it back out of the folder
  // it was in was not offered at all.
  const [destinations, setDestinations] = useState<string[]>([])

  const loadDestinations = useCallback(async () => {
    try {
      const res = await client.directoryTree()
      setDestinations(res?.result ?? [])
    } catch {
      setDestinations([])
    }
  }, [client])

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

  // 删除文件夹。
  //
  // 有内容的文件夹会被服务端拒绝，所以按钮是禁用的；这里的检查是
  // 第二道，因为按钮禁用的理由可能跟用户看到的不一样。
  const removeFolder = async (f: Row) => {
    if ((f.fileCount ?? 0) > 0) {
      pushMessage(i18n.t('folderNotEmpty'), 'error')
      return
    }

    try {
      const res = await fetch(
        `${serverAddr}/filebrowser/folder?path=${encodeURIComponent(f.path)}`,
        { method: 'DELETE' })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      await list(subdir)
    } catch (e) {
      pushMessage(String(e), 'error')
    }
  }

  const selectedFiles = files.filter(f => selected.has(f.path))

  // The actions on a file, in one place so that the buttons next to the name
  // on a phone and the ones in the column on a desktop are the same buttons.
  const actions = {
    onPlay: (f: Row) => setPlaying(f),
    onCaptions: (f: Row) => setCaptioning(f),
    onMove: (f: Row) => {
      setSelected(new Set([f.path]))
      void loadDestinations()
      setFolderDialog('move')
    },
    onDownload: (f: Row) => setDownloading(f),
    onDelete: (f: Row) => { void remove([f.path]) },
  }

  return (
    <Container maxWidth="xl" sx={{ mt: 2, mb: 8 }}>
      <Paper sx={{ p: 2 }}>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
          <Typography variant="h6" color="primary">{i18n.t('browseButtonLabel')}</Typography>

          {/* 新建文件夹：在标题旁边，因为它是在这一层做的事。 */}
          <Tooltip title={i18n.t('folderCreateTitle')}>
            <IconButton size="small" onClick={() => setFolderDialog('create')}>
              <CreateNewFolderIcon fontSize="small" />
            </IconButton>
          </Tooltip>

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
            <Tooltip title={i18n.t('folderMoveTitle')}>
              <span>
                <IconButton
                  size="small"
                  disabled={selectedFiles.length === 0}
                  onClick={() => setFolderDialog('move')}
                >
                  <DriveFileMoveIcon fontSize="small" />
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

        {/* 返回上一级：到父目录，而不是一直回到根。文件夹可以嵌套，
            只回到根会让人以为上面没有别的了。

            顶层没有"上一级"，所以那一层不显示这个按钮 —— 那个父目录是
            下载目录的外面，按下去会列出不属于这里的东西。 */}
        {parent !== '' && (
          <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 1 }}>
            <Button size="small" onClick={() => setSubdir(parent)}>
              ← {i18n.t('goUp')}
            </Button>

            {/* 现在在哪个目录里。文件夹里的列表和外面的长得一样，
                没有这一行就看不出自己在哪。 */}
            <Typography variant="body2" color="text.secondary" noWrap>
              {i18n.t('currentDirectory').replace('{name}', subdirName)}
            </Typography>
          </Stack>
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
                  {/* 来源图标列没有表头文字：图标自己说明了一切，写成
                      "来源"反而占位置。 */}
                  <TableCell sx={{ width: 36 }} />
                  {/* 表头用主题色：这些是列名，和标题一样是该被强调的东西，
                      默认灰色让表格和页面其他部分的配色脱节。 */}
                  {/* 表头不换行：中文的列名只有四个字，换行之后表头比
                      内容还高，看起来是错位的。 */}
                  <TableCell sx={{ color: 'primary.main', whiteSpace: 'nowrap' }}>{i18n.t('colFileName')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main', whiteSpace: 'nowrap' }}>{i18n.t('colDuration')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main', whiteSpace: 'nowrap' }}>{i18n.t('colDownloadTime')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main', whiteSpace: 'nowrap' }}>{i18n.t('colFileSize')}</TableCell>
                  <TableCell align="right" sx={{ color: 'primary.main', whiteSpace: 'nowrap' }}>{i18n.t('colActions')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {folders.map(f => (
                  <TableRow key={f.path} hover>
                    <TableCell padding="checkbox" />

                    {/* 文件夹没有来源站点：它不是从某个网站下来的。 */}
                    <TableCell />

                    <TableCell>
                      {/* 文件夹的名字和视频的名字左对齐。
                          名字在一个按钮里，按钮自带内边距，文字就比
                          同一列里的视频文件名右移了一截，看着像没对齐。
                          所以内边距去掉，间距由外面的 Stack 统一管。 */}
                      <Stack direction="row" spacing={1} alignItems="center">
                        {/* 文件夹是黄色的，跟 Windows 里一样。默认的主题色
                            把它染成了蓝色，一个文件夹看起来就不像文件夹了。 */}
                        <FolderIcon fontSize="small" sx={{ color: '#FFB300' }} />
                        <Button
                          size="small"
                          onClick={() => setSubdir(f.path)}
                          sx={{
                            // 名字保持原样：MUI 的按钮默认把文字转成大写，
                            // 一个叫 "Security" 的文件夹显示成 "SECURITY"
                            // 看着像是另一个东西。
                            textTransform: 'none',
                            // 去掉按钮的内边距，让文字的左边和视频文件名
                            // 的左边在同一条线上。
                            px: 0,
                            minWidth: 0,
                            justifyContent: 'flex-start',
                          }}
                        >
                          {f.name}
                        </Button>
                      </Stack>
                    </TableCell>

                    {/* 文件夹的数字是关于它里面的文件的：有多少个、
                        最新的是什么时候下的、占多大地方。 */}
                    <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                      {i18n.t('folderFileCount').replace('{count}', String(f.fileCount ?? 0))}
                    </TableCell>
                    <TableCell align="right">
                      {f.folderModTime ? new Date(f.folderModTime).toLocaleString() : '—'}
                    </TableCell>
                    <TableCell align="right">{formatSize(f.size)}</TableCell>

                    {/* 文件夹不能播放、不能下字幕、也不能下载：那些都是
                        对单个文件做的事。它能改名字和删除，而有内容的
                        文件夹不能删。 */}
                    <TableCell align="right">
                      <Tooltip title={i18n.t('folderRenameTitle')}>
                        <IconButton
                          size="small"
                          onClick={() => {
                            setRenaming(f)
                            setFolderDialog('rename')
                          }}
                        >
                          <DriveFileRenameOutlineIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>

                      {/* 有内容的文件夹不能删：删掉它就得决定那些文件
                          去哪，而每一种答案都会丢掉用户想要的东西。 */}
                      <Tooltip title={
                        (f.fileCount ?? 0) > 0
                          ? i18n.t('folderNotEmpty')
                          : i18n.t('folderDelete')
                      }>
                        <span>
                          <IconButton
                            size="small"
                            onClick={() => removeFolder(f)}
                            disabled={(f.fileCount ?? 0) > 0}
                          >
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </TableCell>
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
                    {/* 来源站点的图标：文件名看不出是哪个站下来的，
                        一列图标一眼就知道。 */}
                    <TableCell>
                      {/* The span matters: a tooltip puts a ref on its
                          child, and a div does not take one. A <div> here
                          brought the whole page down with a React error
                          that said nothing about what was wrong. */}
                      <Tooltip title={f.sourceSite ?? ''}>
                        <Box
                          component="span"
                          sx={{ display: 'inline-flex', alignItems: 'center' }}
                        >
                          {siteIcon(f.sourceSite, theme.palette.primary.main, isDark(theme))}
                        </Box>
                      </Tooltip>
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
                        {/* 字幕正在下载：跟在文件名后面，等它结束这一行就恢复
                            原样，和别的行一样。 */}
                        {captionJobs[f.path] && (
                          <Chip
                            size="small"
                            color="primary"
                            icon={<CircularProgress size={11} sx={{ color: 'inherit' }} />}
                            label={i18n.t('captionsDownloadingFor').replace(
                              '{count}',
                              String(captionJobs[f.path].languages.length)
                            )}
                            sx={{ height: 20, fontSize: 11 }}
                          />
                        )}

                        {/* 手机上操作按钮在名字旁边。
                            表格比手机的屏幕宽，操作那一列在右边，
                            要划过去才够得着播放按钮 —— 而手本来
                            就在这一行的名字上。宽屏时这一份不显示，
                            那一列里有一份。 */}
                        <Box sx={{ display: { xs: 'flex', md: 'none' }, ml: 'auto' }}>
                          <FileActions file={f} {...actions} />
                        </Box>
                      </Stack>
                    </TableCell>
                    <TableCell align="right">{formatDuration(f.duration) || '—'}</TableCell>
                    <TableCell align="right">
                      {f.modTime ? new Date(f.modTime).toLocaleString() : '—'}
                    </TableCell>
                    <TableCell align="right">{formatSize(f.size)}</TableCell>
                    {/* 宽屏时操作在这一列；窄屏时它们在名字旁边，
                        见上面那一处。 */}
                    <TableCell align="right" sx={{ display: { xs: 'none', md: 'table-cell' } }}>
                      <FileActions file={f} {...actions} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>

      <FolderDialog
        open={folderDialog !== null}
        mode={folderDialog ?? 'create'}
        parent={subdir}
        destinations={destinations}
        onOpenMove={() => { void loadDestinations() }}
        files={selectedFiles.map(f => f.path)}
        folder={renaming ?? undefined}
        onClose={() => { setFolderDialog(null); setRenaming(null) }}
        onDone={() => { void list(subdir); setSelected(new Set()); setRenaming(null) }}
      />

      <DownloadDialog
        open={downloading !== null}
        path={downloading?.path ?? ''}
        name={downloading?.name ?? ''}
        onClose={() => setDownloading(null)}
      />

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
        videoCodec={playing?.videoCodec}
        audioCodec={playing?.audioCodec}
        onClose={() => setPlaying(null)}
      />
    </Container>
  )
}
