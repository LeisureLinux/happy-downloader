import CookieIcon from '@mui/icons-material/Cookie'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
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
//
// The import the user uploaded is shown instead when there is one, because
// that is the cookie the downloads are actually using.
//
// The warning is for the automatic export: it is supposed to be replaced every
// ten minutes, so a timestamp that has not moved for longer than that means
// the refresher is not running — and the symptom is a download failing with a
// bot wall, which looks like nothing at all unless something says so.
const STALE_AFTER_MINUTES = 30

const CookieIndicator: React.FC = () => {
  const status = useAtomValue(cookieStatusState)
  const { i18n } = useI18n()

  const imported = status.imported

  // The imported export is in use: it is the cookie the downloads use, so it
  // is what the timestamp has to be about.
  if (imported) {
    return (
      <Tooltip title={`${i18n.t('cookieModifiedAt')} · ${imported.path}`}>
        <Box
          component="span"
          sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'primary.main' }}
        >
          <CookieIcon fontSize="small" />
          <Box component="span">{formatTimestamp(imported.modifiedAt)}</Box>
        </Box>
      </Tooltip>
    )
  }

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

  // The automatic export, and whether it looks alive.
  const stale = isStale(status.modifiedAt)

  return (
    <Tooltip
      title={`${i18n.t('cookieModifiedAt')} · ${status.sourcePath ?? ''}`}
    >
      <Box
        component="span"
        sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'primary.main' }}
      >
        <CookieIcon fontSize="small" />
        <Box component="span">{formatTimestamp(status.modifiedAt)}</Box>
        {stale && (
          <WarningAmberIcon
            fontSize="small"
            sx={{ color: 'warning.main' }}
          />
        )}
      </Box>
    </Tooltip>
  )
}

// isStale reports whether the automatic export has not been replaced for
// longer than it should have been.
//
// The refresher runs every ten minutes. A quarter of an hour of quiet is
// therefore not a schedule: something is wrong, and the only visible symptom
// is a download that fails with a bot wall.
const isStale = (modifiedAt?: string): boolean => {
  if (!modifiedAt) return true

  const then = new Date(modifiedAt).getTime()
  if (Number.isNaN(then)) return true

  return Date.now() - then > STALE_AFTER_MINUTES * 60 * 1000
}

export default CookieIndicator
