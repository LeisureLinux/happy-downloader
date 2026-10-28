import DeleteIcon from '@mui/icons-material/Delete'
import DownloadDoneIcon from '@mui/icons-material/DownloadDone'
import ReplayIcon from '@mui/icons-material/Replay'
import PauseIcon from '@mui/icons-material/Pause'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import {
  Box,
  Button,
  ButtonGroup,
  Chip,
  IconButton,
  LinearProgress,
  LinearProgressProps,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography
} from "@mui/material"
import { forwardRef, useMemo, useState } from 'react'
import { TableComponents, TableVirtuoso } from 'react-virtuoso'
import { activeDownloadsState } from '../atoms/downloads'
import { serverURL } from '../atoms/settings'
import { useRPC } from '../hooks/useRPC'
import { ProcessStatus, RPCResult } from '../types'
import { base64URLEncode, formatSize, formatSpeedMiB } from "../utils"
import { useI18n } from '../hooks/useI18n'
import { useAtomValue } from 'jotai'

const columns = [
  {
    width: 420,
    labelKey: 'colFile',
    dataKey: 'file',
    numeric: false,
  },
  {
    width: 90,
    labelKey: 'colFileFormat',
    dataKey: 'format',
    numeric: false,
  },
  {
    width: 130,
    labelKey: 'colResolution',
    dataKey: 'resolution',
    numeric: true,
  },
  {
    width: 160,
    labelKey: 'colProgress',
    dataKey: 'progress',
    numeric: true,
  },
  {
    width: 160,
    labelKey: 'colAddedOn',
    dataKey: 'addedon',
    numeric: true,
  },
  {
    width: 150,
    labelKey: 'colActions',
    dataKey: 'actions',
    numeric: true,
  },
] as const

function LinearProgressWithLabel(props: LinearProgressProps & { value: number }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center' }}>
      <Box sx={{ width: '100%', mr: 1 }}>
        <LinearProgress variant="determinate" {...props} />
      </Box>
      <Box sx={{ minWidth: 35 }}>
        <Typography variant="body2" color="text.secondary">{`${Math.round(
          props.value,
        )}%`}</Typography>
      </Box>
    </Box>
  )
}

const VirtuosoTableComponents: TableComponents<RPCResult> = {
  Scroller: forwardRef<HTMLDivElement>((props, ref) => (
    <TableContainer {...props} ref={ref} />
  )),
  Table: (props) => (
    <Table {...props} sx={{ borderCollapse: 'separate', tableLayout: 'fixed', mt: 2 }} size='small' />
  ),
  TableHead,
  TableRow: ({ item: _item, ...props }) => <TableRow {...props} />,
  TableBody: forwardRef<HTMLTableSectionElement>((props, ref) => (
    <TableBody {...props} ref={ref} />
  )),
}



const DownloadsTableView: React.FC = () => {
  const allDownloads = useAtomValue(activeDownloadsState)
  const serverAddr = useAtomValue(serverURL)
  const { client } = useRPC()
  const { i18n } = useI18n()

  // This is a queue, not a history. A download that finished is a file, and
  // files belong in the browser; leaving them here meant the list grew without
  // limit and the thing you were waiting for was buried in things you already
  // had. Finished entries move themselves out by not being listed.
  const downloads = useMemo(
    () => allDownloads.filter(d => d.progress.process_status !== ProcessStatus.COMPLETED),
    [allDownloads]
  )

  const failedDownloads = useMemo(
    () => downloads.filter(d => d.progress.process_status === ProcessStatus.ERRORED),
    [downloads]
  )

  // A download that has been queued but has not transferred anything yet shows
  // a 0% bar, which is indistinguishable from a stuck one. Naming the stage
  // says whether to keep waiting.
  const statusLabel = (r: RPCResult) => {
    switch (r.progress.process_status) {
      case ProcessStatus.PENDING:
        return i18n.t('statusPending')
      case ProcessStatus.DOWNLOADING:
        return r.progress.percentage
          ? i18n.t('statusDownloading')
          : i18n.t('statusMetadata')
      case ProcessStatus.COMPLETED:
        return i18n.t('statusCompleted')
      case ProcessStatus.ERRORED:
        return i18n.t('statusErrored')
      case ProcessStatus.PAUSED:
        return i18n.t('statusPaused')
      default:
        return ''
    }
  }

  // The list outlives the files: an entry whose file was removed elsewhere has
  // nothing to play or download, and offering the buttons produces a 404.
  const fileOnDisk = (r: RPCResult) => r.output.fileExists === true
  // The saved file's own suffix, which is what the user opens. Falls back to
  // the container yt-dlp reported before the file exists.
  const fileFormat = (r: RPCResult): string => {
    const saved = r.output.savedFilePath ?? ''
    const dot = saved.lastIndexOf('.')
    if (dot > 0) return saved.slice(dot + 1).toLowerCase()
    return (r.info.ext ?? '').replace(/^\./, '').toLowerCase()
  }

  // Only the size on disk. The metadata estimate is the source's *best video*
  // format, so an audio-only download showed 35 MB for a 9 MB mp3 and a 360p
  // cap showed the 1080p figure. No number is better than a wrong one.
  const actualSize = (r: RPCResult) =>
    r.output.fileSize && r.output.fileSize > 0 ? r.output.fileSize : 0


  const viewFile = (path: string) => {
    const encoded = base64URLEncode(path)
    window.open(`${serverAddr}/filebrowser/v/${encoded}`)
  }

  const downloadFile = (path: string) => {
    const encoded = base64URLEncode(path)
    window.open(`${serverAddr}/filebrowser/d/${encoded}`)
  }

  // 重新入队。服务端复用原来的参数，所以重试的是同一次请求，
  // 不是重新猜一遍要什么格式。
  const resume = (r: RPCResult) => client.retry(r.id)

  // A paused download is resumed rather than retried: it is the same
  // download, and yt-dlp continues the partial file it left behind, so it
  // picks up where it stopped instead of starting over.
  const continuePaused = (r: RPCResult) => client.resume(r.id)

  // Stopping keeps the entry in the queue. It used to remove it, which lost
  // the row and put a file that was never written into the file browser.
  const stop = (r: RPCResult) => r.progress.process_status === ProcessStatus.COMPLETED
    ? client.clear(r.id)
    : client.pause(r.id)

  // process_status is authoritative. percentage === '-1' only matched items
  // that never reported progress, so completed downloads kept the stop icon
  // and hid the view/download actions.
  const isDone = (r: RPCResult) =>
    r.progress.process_status === ProcessStatus.COMPLETED


  function rowContent(_index: number, download: RPCResult) {
    const done = isDone(download)

    return (
      <>
        {/* 文件名与大小在同一列：大小是文件的属性，单独一列会让名称列被压缩 */}
        <TableCell>
          <Typography variant="body2" noWrap sx={{ maxWidth: 380 }}>
            {download.info.title}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {/* 用磁盘上的真实大小。元数据里的 filesize_approx 是源站最佳格式的
                估算，加了分辨率上限或只下音频之后与实际文件差很远。 */}
            {formatSize(actualSize(download))}
            {!fileOnDisk(download) && done && (
              <Box component="span" sx={{ ml: 1, color: 'error.main' }}>
                ({i18n.t('fileMissing')})
              </Box>
            )}
          </Typography>
        </TableCell>
        <TableCell>
          {/* 后缀取自实际保存的文件；还没落盘时退回元数据里的容器。
              用户看到的和磁盘上的一致，不用去猜 -f 到底选了什么。 */}
          <Chip
            size="small"
            variant="outlined"
            label={fileFormat(download) || '—'}
          />
        </TableCell>
        <TableCell align="right">
          {download.output.height
            ? `${download.output.width ?? ''}x${download.output.height}`
            : '—'}
        </TableCell>
        <TableCell align="right">
          {/* 状态文字：点下"开始"到真正开始传数据之间有一段等待（排队、取元数据），
              只显示一个 0% 的进度条会让人以为没反应。 */}
          <Typography variant="caption" color="text.secondary" display="block">
            {statusLabel(download)}
          </Typography>
          <LinearProgressWithLabel
            sx={{ height: '16px', mt: 0.5 }}
            value={done ? 100 : Number(download.progress.percentage.replace('%', '') || 0)}
            variant={download.progress.process_status === 0 ? 'indeterminate' : 'determinate'}
            color={'primary'}
          />
        </TableCell>
        <TableCell align="right">
          {new Date(download.info.created_at).toLocaleString()}
        </TableCell>
        <TableCell align="right">
          <ButtonGroup size="small">
            {download.progress.process_status === ProcessStatus.ERRORED ? (
              <>
                {/* 失败的行留着是为了还能重来，所以第一个按钮必须是"重来"，
                    而不是"播放"——队列里没有可播放的东西。 */}
                <Tooltip title={i18n.t('resumeDownload')}>
                  <IconButton size="small" color="primary" onClick={() => resume(download)}>
                    <ReplayIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={i18n.t('removeFromQueue')}>
                  <IconButton size="small" onClick={() => client.clear(download.id)}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </>
            ) : download.progress.process_status === ProcessStatus.PAUSED ? (
              <>
                {/* 暂停的行：继续和删除。继续是真的继续——yt-dlp 留着
                    下载了一半的文件，所以它接着下，而不是从头再来。 */}
                <Tooltip title={i18n.t('continueDownload')}>
                  <IconButton size="small" color="primary" onClick={() => continuePaused(download)}>
                    <PlayArrowIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={i18n.t('removeFromQueue')}>
                  <IconButton size="small" onClick={() => client.clear(download.id)}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </>
            ) : (
              <>
                {/* A running download can be paused and it can be removed.
                    Pausing leaves it in the queue — the partial file is
                    kept and it can be continued. Removing is the other
                    thing: a download that is going nowhere should be
                    able to leave, and with only a pause button there was
                    no way to do it. */}
                <Tooltip title={i18n.t('pauseDownload')}>
                  <IconButton size="small" onClick={() => stop(download)}>
                    <PauseIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={i18n.t('removeFromQueue')}>
                  <IconButton size="small" onClick={() => client.clear(download.id)}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </>
            )}
          </ButtonGroup>
        </TableCell>
      </>
    )
  }

  return (
    <Box sx={{ width: '100%' }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
        <Typography variant="h6" color="primary">{i18n.t('downloadQueue')}</Typography>
        <Typography variant="caption" color="text.secondary">
          {downloads.length === 0
            ? i18n.t('queueEmpty')
            : i18n.t('queueHint')}
        </Typography>
        <Box flexGrow={1} />
        {failedDownloads.length > 0 && (
          <Button
            size="small"
            color="error"
            startIcon={<DeleteIcon />}
            onClick={() => failedDownloads.forEach(d => client.clear(d.id))}
          >
            {i18n.t('clearFailedButton')}
          </Button>
        )}
      </Stack>

      <Box style={{ height: downloads.length === 0 ? '0vh' : '80vh', width: '100%' }}>
        <TableVirtuoso
          hidden={downloads.length === 0}
          data={downloads}
          components={VirtuosoTableComponents}
          fixedHeaderContent={() => (
            <TableRow>
              {columns.map((column) => (
                <TableCell
                  key={column.dataKey}
                  variant="head"
                  align={column.numeric || false ? 'right' : 'left'}
                  style={{ width: column.width }}
                >
                  {'labelKey' in column ? i18n.t(column.labelKey) : ''}
                </TableCell>
              ))}
            </TableRow>
          )}
          itemContent={rowContent}
        />
      </Box>
    </Box>
  )
}

export default DownloadsTableView