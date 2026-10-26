import CloseIcon from '@mui/icons-material/Close'
import { Box, IconButton, Paper, Tooltip, Typography } from '@mui/material'
import { useAtom } from 'jotai'
import { settingsPanelOpenAtom } from '../atoms/ui'
import { useI18n } from '../hooks/useI18n'
import { SettingsContent } from '../views/Settings'

// Settings, as a panel rather than a page.
//
// Most things here are changed while watching what they do to something else —
// a download in flight, a probe that is slow. A page would replace the thing you
// are watching with a form, and then replace it again on the way back. An
// overlay leaves it visible.
const SettingsPanel: React.FC<{ open: boolean; onClose: () => void }> = ({ open, onClose }) => {
  const [, setOpen] = useAtom(settingsPanelOpenAtom)
  const { i18n } = useI18n()

  if (!open) return null

  const close = () => {
    setOpen(false)
    onClose()
  }

  return (
    <Paper
      elevation={12}
      sx={{
        position: 'fixed',
        top: 76,
        bottom: 72,
        width: '80vw',
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 1300,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      <Box sx={{
        display: 'flex', alignItems: 'center', gap: 1,
        px: 2, py: 1, borderBottom: 1, borderColor: 'divider',
      }}>
        <Typography variant="h6" color="primary" sx={{ flexGrow: 1 }}>
          {i18n.t('settingsAnchor')}
        </Typography>
        <Tooltip title={i18n.t('close')}>
          <IconButton size="small" onClick={close}>
            <CloseIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>
      <Box sx={{ flexGrow: 1, overflow: 'auto' }}>
        <SettingsContent />
      </Box>
    </Paper>
  )
}

export default SettingsPanel
