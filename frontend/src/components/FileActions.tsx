import DeleteIcon from '@mui/icons-material/Delete'
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMove'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import SmartDisplayIcon from '@mui/icons-material/SmartDisplay'
import SubtitleIcon from '@mui/icons-material/Subtitles'
import { ButtonGroup, IconButton, Tooltip } from '@mui/material'
import { useI18n } from '../hooks/useI18n'
import { DirectoryEntry } from '../types'

// The buttons that act on one file.
//
// They are a component because they appear twice: in the actions column on a
// screen with room for it, and next to the file's name on a phone.
//
// The phone is why. The table is wider than a phone's screen, so the actions
// column is off to the right and has to be scrolled to — which is a lot of
// scrolling to reach the play button of the file you are looking at. On a
// narrow screen the buttons are next to the name instead, where the finger
// already is.
//
// A folder has no such buttons: it is not a video, so playing it, giving it
// captions and downloading it do not mean anything.

// The file and the handlers are generic over the file's type, so the row
// type of the file browser — which carries the selection — can be passed
// without being widened and narrowed again.
type Props<T> = {
  file: T
  onPlay: (f: T) => void
  onCaptions: (f: T) => void
  onMove: (f: T) => void
  onDownload: (f: T) => void
  onDelete: (f: T) => void
}

const FileActions = <T extends DirectoryEntry>({
  file, onPlay, onCaptions, onMove, onDownload, onDelete,
}: Props<T>) => {
  const { i18n } = useI18n()

  return (
    // 与下载历史用同一组图标按钮：同一类操作在两处长得一样
    <ButtonGroup size="small">
      <Tooltip title={i18n.t('playSelected')}>
        <IconButton size="small" onClick={() => onPlay(file)}>
          <SmartDisplayIcon fontSize="small" />
        </IconButton>
      </Tooltip>

      {/* 字幕单独下载：YouTube 限流字幕端点比视频严格得多，
          把它和视频绑在一起会让限流毁掉整个下载。 */}
      {file.isVideo && (
        <Tooltip title={i18n.t('captionsButton')}>
          <IconButton size="small" onClick={() => onCaptions(file)}>
            <SubtitleIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      )}

      {/* 移动到文件夹。 */}
      <Tooltip title={i18n.t('folderMoveTitle')}>
        <IconButton size="small" onClick={() => onMove(file)}>
          <DriveFileMoveIcon fontSize="small" />
        </IconButton>
      </Tooltip>

      {/* 下载按钮给出选择：视频本身，和每一个字幕文件。
          以前只有一个按钮，产出的是一个 zip——手机上收到
          zip 是没法播放的。 */}
      <Tooltip title={i18n.t('downloadSelected')}>
        <IconButton size="small" onClick={() => onDownload(file)}>
          <FileDownloadIcon fontSize="small" />
        </IconButton>
      </Tooltip>

      <Tooltip title={i18n.t('deleteSelected')}>
        <IconButton size="small" onClick={() => onDelete(file)}>
          <DeleteIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </ButtonGroup>
  )
}

export default FileActions
