import DownloadIcon from '@mui/icons-material/Download'
import GitHubIcon from '@mui/icons-material/GitHub'
import SettingsEthernet from '@mui/icons-material/SettingsEthernet'
import { AppBar, Box, Divider, IconButton, Toolbar, Tooltip } from '@mui/material'
import { Suspense } from 'react'
import { settingsState } from '../atoms/settings'
import { connectedState } from '../atoms/status'
import { totalDownloadSpeedState } from '../atoms/ui'
import { useI18n } from '../hooks/useI18n'
import { formatSpeedMiB } from '../utils'
import FreeSpaceIndicator from './FreeSpaceIndicator'
import { useAtomValue } from 'jotai'

const Footer: React.FC = () => {
  const settings = useAtomValue(settingsState)
  const isConnected = useAtomValue(connectedState)
  const totalDownloadSpeed = useAtomValue(totalDownloadSpeedState)

  const mode = settings.theme
  const { i18n } = useI18n()

  return (
    <AppBar position="fixed" color="default" sx={{
      top: 'auto',
      bottom: 0,
      height: 48,
      zIndex: 1200,
      borderTop: mode === 'light'
        ? '1px solid rgba(0, 0, 0, 0.12)'
        : '1px solid rgba(255, 255, 255, 0.12)',
    }}>
      {/* paddingBottom made the toolbar 64px tall inside a 48px bar, so the
          row overflowed the viewport and the icons were clipped at the bottom
          edge. The bar centres its own content; nothing needs to be pushed. */}
      <Toolbar sx={{
        minHeight: '48px !important',
        fontSize: 14,
        display: 'flex', gap: 1, justifyContent: 'space-between'
      }}>
        <div style={{ display: 'flex', gap: 4, 'alignItems': 'center' }}>
          {/* 仓库入口放在最左下角：它是"这个部署是什么"的入口，
              不是导航，所以和状态信息排在一起而不是顶栏。 */}
          <Tooltip title={i18n.t('sourceCode')}>
            <IconButton
              size="small"
              component="a"
              href="https://github.com/LeisureLinux/happy-downloader"
              target="_blank"
              rel="noopener noreferrer"
              sx={{ color: 'primary.main' }}
            >
              <GitHubIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Divider orientation="vertical" flexItem />
          <div style={{
            display: 'flex',
            alignItems: 'center',
            flexWrap: 'wrap',
            marginRight: 'px',
            gap: 3,
          }}>
            <DownloadIcon sx={{ color: 'primary.main' }} />
            <Box component="span" sx={{ color: 'primary.main' }}>
              {formatSpeedMiB(totalDownloadSpeed)}
            </Box>
            <Divider orientation="vertical" flexItem />
            <SettingsEthernet sx={{ color: 'primary.main' }} />
            <Box component="span" sx={{ color: 'primary.main' }}>
              {isConnected ? settings.serverAddr : i18n.t('notConnectedText')}
            </Box>
          </div>
          <Divider orientation="vertical" flexItem />
          <Suspense fallback={i18n.t('loadingLabel')}>
            <FreeSpaceIndicator />
          </Suspense>
        </div>
      </Toolbar>
    </AppBar>
  )
}

export default Footer