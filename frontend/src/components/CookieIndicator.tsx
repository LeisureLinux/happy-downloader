import CookieIcon from '@mui/icons-material/Cookie'
import { Box, Tooltip } from '@mui/material'
import { cookieStatusState } from '../atoms/settings'
import { formatTimestamp } from '../utils'
import { useI18n } from '../hooks/useI18n'
import { useAtomValue } from 'jotai'

// When the cookie export was last replaced, next to the disk figure.
//
// A download that fails with a bot wall is almost always a stale credential,
// and the answer used to be three clicks deep in settings. The file is
// rewritten by a timer every ten minutes, so this number moving is also the
// only outside sign that the refresher is still working: when the timer's unit
// pointed at a directory that had been renamed, the timestamp froze and nothing
// else said anything was wrong.
const CookieIndicator: React.FC = () => {
  const status = useAtomValue(cookieStatusState)
  const { i18n } = useI18n()

  if (!status.exists) {
    return (
      <Tooltip title={i18n.t('cookieNotLoaded')}>
        <Box
          component="span"
          sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'text.disabled' }}
        >
          <CookieIcon fontSize="small" />
          <Box component="span">—</Box>
        </Box>
      </Tooltip>
    )
  }

  return (
    <Tooltip title={`${i18n.t('cookieModifiedAt')} · ${status.sourcePath ?? ''}`}>
      <Box
        component="span"
        sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'primary.main' }}
      >
        <CookieIcon fontSize="small" />
        <Box component="span">{formatTimestamp(status.modifiedAt)}</Box>
      </Box>
    </Tooltip>
  )
}

export default CookieIndicator
