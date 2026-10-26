import { useEffect, useRef, useState } from 'react'
import { Box, IconButton, Paper, Tooltip, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import TerminalIcon from '@mui/icons-material/Terminal'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useAtomValue } from 'jotai'

// The log, as a panel rather than a page.
//
// A page is the wrong shape for this: reading the log is something done *while*
// watching a download, and navigating away loses the download list behind it.
// The panel overlays, so both are visible at once.
//
// Newest first. The server sends its buffered history when the stream opens, so
// the panel starts with the lines that explain why it was opened instead of an
// empty box.
const LogPanel: React.FC<{ open: boolean; onClose: () => void }> = ({ open, onClose }) => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()

  const [lines, setLines] = useState<string[]>([])
  const [connected, setConnected] = useState(false)
  const boxRef = useRef<HTMLDivElement>(null)
  const stickToTop = useRef(true)

  useEffect(() => {
    if (!open) return

    const es = new EventSource(`${serverAddr}/log/sse`)

    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)

    es.addEventListener('log', event => {
      try {
        const msg: string = JSON.parse(event.data)
        setLines(prev => [msg, ...prev].slice(0, 500))
        if (stickToTop.current) boxRef.current?.scrollTo(0, 0)
      } catch {
        // A malformed frame is not worth breaking the stream over.
      }
    })

    return () => {
      es.close()
      setConnected(false)
    }
  }, [open, serverAddr])

  if (!open) return null

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
      <Box
        sx={{
          display: 'flex', alignItems: 'center', gap: 1,
          px: 1.5, py: 0.75, borderBottom: 1, borderColor: 'divider',
        }}
      >
        <Typography variant="h6" color="primary" sx={{ flexGrow: 1 }}>
          {i18n.t('logsTitle')}
        </Typography>
        {/* 一个在跳的圆点：日志不动的时候，至少能看出连接还活着。 */}
        <Box
          sx={{
            width: 8, height: 8, borderRadius: '50%',
            bgcolor: connected ? 'success.main' : 'error.main',
            animation: connected ? 'logPulse 1.6s ease-in-out infinite' : 'none',
            '@keyframes logPulse': {
              '0%, 100%': { opacity: 1, transform: 'scale(1)' },
              '50%': { opacity: 0.35, transform: 'scale(0.75)' },
            },
          }}
        />
        <Typography variant="caption" color="text.secondary">
          {lines.length}
        </Typography>
        <Tooltip title={i18n.t('close')}>
          <IconButton size="small" onClick={onClose}>
            <CloseIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Box
        ref={boxRef}
        onScroll={e => {
          // Keep following only while the reader is at the newest end.
          stickToTop.current = e.currentTarget.scrollTop <= 2
        }}
        sx={{
          flexGrow: 1,
          overflow: 'auto',
          bgcolor: 'black',
          color: 'white',
          p: 1,
          fontFamily: 'Roboto Mono',
          fontSize: '12px',
          fontWeight: 600,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {lines.length === 0 && (
          <Typography variant="caption" sx={{ color: 'grey.500' }}>
            {connected ? i18n.t('awaitingLogs') : '…'}
          </Typography>
        )}
        {lines.map((line, i) => (
          <div
            key={i}
            style={{
              color: line.includes('level=ERROR') ? '#ff6b6b'
                : line.includes('level=WARN') ? '#ffb86b'
                  : undefined,
            }}
          >
            {line}
          </div>
        ))}
      </Box>
    </Paper>
  )
}

export default LogPanel
