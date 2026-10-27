import SystemUpdateAltIcon from '@mui/icons-material/SystemUpdateAlt'
import { Box, Tooltip } from '@mui/material'
import { ytdlpRpcVersionState } from '../atoms/status'
import { useAtomValue } from 'jotai'

// The yt-dlp version, next to the disk figure and the cookie's age.
//
// yt-dlp breaks YouTube roughly monthly, and the fix is a new binary rather
// than a new build of this application. So "which yt-dlp is this" is a
// question worth answering on the page: the version is in the footer, and the
// tooltip says where the binary is, because a host can have several and a
// path is what identifies one.
//
// The version atom is a Promise atom, so reading it needs a Suspense
// boundary; the footer's is the one that provides it.
const YtdlpIndicator: React.FC = () => {
  const version = useAtomValue(ytdlpRpcVersionState)

  if (!version.ytdlpVersion || version.ytdlpVersion === 'unknown version') {
    return null
  }

  return (
    <Tooltip title={version.ytdlpPath ?? version.ytdlpVersion}>
      <Box
        component="span"
        sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'primary.main' }}
      >
        <SystemUpdateAltIcon fontSize="small" />
        <Box component="span">{version.ytdlpVersion}</Box>
      </Box>
    </Tooltip>
  )
}

export default YtdlpIndicator
