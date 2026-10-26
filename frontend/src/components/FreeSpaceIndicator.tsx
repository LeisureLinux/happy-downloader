import StorageIcon from '@mui/icons-material/Storage'
import { Box } from '@mui/material'
import { freeSpaceBytesState } from '../atoms/status'
import { formatSize } from '../utils'
import { useAtomValue } from 'jotai'

const FreeSpaceIndicator = () => {
  const freeSpace = useAtomValue(freeSpaceBytesState)

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      flexWrap: 'wrap',
      gap: 3
    }}>
      <StorageIcon sx={{ color: 'primary.main' }} />
      <Box component="span" sx={{ color: 'primary.main' }}>
        {formatSize(freeSpace)}
      </Box>
    </div>
  )
}

export default FreeSpaceIndicator