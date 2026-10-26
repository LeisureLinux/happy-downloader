import DeleteIcon from '@mui/icons-material/Delete'
import DownloadIcon from '@mui/icons-material/Download'
import DownloadDoneIcon from '@mui/icons-material/DownloadDone'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import SmartDisplayIcon from '@mui/icons-material/SmartDisplay'
import StopCircleIcon from '@mui/icons-material/StopCircle'
import {
  Box,
  Button,
  ButtonGroup,
  Checkbox,
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
    width: 44,
    label: '',
    dataKey: 'select',
    numeric: false,
  },
  {
    width: 420,
    labelKey: 'colFile',
    dataKey: 'file',
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
  const downloads = useAtomValue(activeDownloadsState)
  const serverAddr = useAtomValue(serverURL)
  const { client } = useRPC()
  const { i18n } = useI18n()

  // Selection applies to completed downloads only: an in-flight one has no
  // file to act on yet.
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const doneDownloads = useMemo(
    () => downloads.filter(d => d.progress.process_status === ProcessStatus.COMPLETED),
    [downloads]
  )

  const toggle = (id: string) => setSelected(prev => {
    const next = new Set(prev)
    next.has(id) ? next.delete(id) : next.add(id)
    return next
  })

  // The list outlives the files: an entry whose file was removed elsewhere has
  // nothing to play or download, and offering the buttons produces a 404.
  const fileOnDisk = (r: RPCResult) => r.output.fileExists === true
  const actualSize = (r: RPCResult) =>
    r.output.fileSize && r.output.fileSize > 0
      ? r.output.fileSize
      : (r.info.filesize_approx ?? 0)

  const allSelected = doneDownloads.length > 0 && doneDownloads.every(d => selected.has(d.id))

  const toggleAll = () => setSelected(
    allSelected ? new Set() : new Set(doneDownloads.map(d => d.id))
  )

  const viewFile = (path: string) => {
    const encoded = base64URLEncode(path)
    window.open(`${serverAddr}/filebrowser/v/${encoded}?token=${localStorage.getItem('token')}`)
  }

  const downloadFile = (path: string) => {
    const encoded = base64URLEncode(path)
    window.open(`${serverAddr}/filebrowser/d/${encoded}?token=${localStorage.getItem('token')}`)
  }

  const stop = (r: RPCResult) => r.progress.process_status === ProcessStatus.COMPLETED
    ? client.clear(r.id)
    : client.kill(r.id)

  // process_status is authoritative. percentage === '-1' only matched items
  // that never reported progress, so completed downloads kept the stop icon
  // and hid the view/download actions.
  const isDone = (r: RPCResult) =>
    r.progress.process_status === ProcessStatus.COMPLETED


  function rowContent(_index: number, download: RPCResult) {
    const done = isDone(download)

    return (
      <>
        <TableCell padding="checkbox">
          {done && (
            <Checkbox
              size="small"
              checked={selected.has(download.id)}
              onChange={() => toggle(download.id)}
            />
          )}
        </TableCell>
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
        <TableCell align="right">
          {download.output.height
            ? `${download.output.width ?? ''}x${download.output.height}`
            : '—'}
        </TableCell>
        <TableCell align="right">
          <LinearProgressWithLabel
            sx={{ height: '16px' }}
            value={done ? 100 : Number(download.progress.percentage.replace('%', ''))}
            variant={download.progress.process_status === 0 ? 'indeterminate' : 'determinate'}
            color={'primary'}
          />
        </TableCell>
        <TableCell align="right">
          {new Date(download.info.created_at).toLocaleString()}
        </TableCell>
        <TableCell align="right">
          <ButtonGroup size="small">
            {done ? (
              <>
                {/* 文件已被删除时只给移除记录的按钮：播放和下载会 404 */}
                {fileOnDisk(download) && (
                  <>
                    <Tooltip title={i18n.t('playSelected')}>
                      <IconButton size="small" onClick={() => viewFile(download.output.savedFilePath)}>
                        <SmartDisplayIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title={i18n.t('downloadSelected')}>
                      <IconButton size="small" onClick={() => downloadFile(download.output.savedFilePath)}>
                        <FileDownloadIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </>
                )}
                <Tooltip title={i18n.t('deleteSelected')}>
                  <IconButton size="small" onClick={() => stop(download)}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </>
            ) : (
              <Tooltip title={i18n.t('stopDownload')}>
                <IconButton size="small" onClick={() => stop(download)}>
                  <StopCircleIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
          </ButtonGroup>
        </TableCell>
      </>
    )
  }

  const selectedDownloads = doneDownloads.filter(d => selected.has(d.id))

  return (
    <Box sx={{ width: '100%' }}>
      {/* 批量操作：作用于勾选的已完成下载 */}
      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
        <Typography variant="h6">{i18n.t('downloadHistory')}</Typography>
        <Box flexGrow={1} />
        <Button
          size="small"
          startIcon={<FileDownloadIcon />}
          disabled={selectedDownloads.length === 0}
          onClick={() => selectedDownloads.forEach(d => downloadFile(d.output.savedFilePath))}
        >
          {i18n.t('downloadSelected')}
        </Button>
        <Button
          size="small"
          startIcon={<SmartDisplayIcon />}
          disabled={selectedDownloads.length === 0}
          onClick={() => selectedDownloads.forEach(d => viewFile(d.output.savedFilePath))}
        >
          {i18n.t('playSelected')}
        </Button>
        <Button
          size="small"
          color="error"
          startIcon={<DeleteIcon />}
          disabled={selectedDownloads.length === 0}
          onClick={() => selectedDownloads.forEach(d => stop(d))}
        >
          {i18n.t('deleteSelected')}
        </Button>
      </Stack>

      <Box style={{ height: downloads.length === 0 ? '0vh' : '80vh', width: '100%' }}>
        <TableVirtuoso
          hidden={downloads.length === 0}
          data={downloads}
          components={VirtuosoTableComponents}
          fixedHeaderContent={() => (
            <TableRow>
              <TableCell padding="checkbox" variant="head">
                <Checkbox
                  size="small"
                  checked={allSelected}
                  indeterminate={!allSelected && selected.size > 0}
                  onChange={toggleAll}
                />
              </TableCell>
              {columns.slice(1).map((column) => (
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