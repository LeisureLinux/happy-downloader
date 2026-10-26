import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import { Box, SvgIcon, Typography } from '@mui/material'
import { activeDownloadsState } from '../atoms/downloads'
import { useI18n } from '../hooks/useI18n'
import { ProcessStatus } from '../types'
import { useAtomValue } from 'jotai'

// The empty state for the download queue.
//
// It used to be a 200px icon over 36px text filling 80vh — a whole screen spent
// saying "nothing here", which pushed the page around every time the queue
// emptied. An empty list is not an announcement; it is a quiet line under the
// form that disappears the moment something is queued.
export default function Splash() {
  const { i18n } = useI18n()
  const allDownloads = useAtomValue(activeDownloadsState)

  // Same notion of "in the queue" the table uses: finished downloads live in
  // the file browser, so they do not count as activity here.
  const queued = allDownloads.filter(d => d.progress.process_status !== ProcessStatus.COMPLETED)

  if (queued.length !== 0) {
    return null
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 1,
        py: 3,
        color: 'text.secondary',
      }}
    >
      <SvgIcon sx={{ fontSize: 20, opacity: 0.6 }}>
        <CloudDownloadIcon />
      </SvgIcon>
      <Typography variant="body2">{i18n.t('splashText')}</Typography>
    </Box>
  )
}
