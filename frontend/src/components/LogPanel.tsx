import { useEffect, useRef, useState } from 'react'
import { Box, IconButton, Paper, Tooltip, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import PauseIcon from '@mui/icons-material/Pause'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
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
  const [paused, setPaused] = useState(false)
  const boxRef = useRef<HTMLDivElement>(null)
  const stickToTop = useRef(true)
  // The paused flag is read inside the event handler, which is installed
  // once. A ref keeps it current without making the stream reconnect — and
  // reconnecting would send the buffered history again, which would show
  // every line twice.
  const pausedRef = useRef(false)
  pausedRef.current = paused
  // The lines that arrived while the panel was paused. They are kept out of
  // the list until the reader resumes, because the list is rendered newest
  // first: putting a line in at the top shifts everything below it, so the
  // text the reader is looking at moves away even though nothing scrolled.
  // Holding the lines back is what actually freezes the view.
  const pendingRef = useRef<string[]>([])

  useEffect(() => {
    if (!open) return

    const es = new EventSource(`${serverAddr}/log/sse`)

    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)

    es.addEventListener('log', event => {
      try {
        const msg: string = JSON.parse(event.data)

        if (pausedRef.current) {
          // Held back until the reader resumes. The count in the header
          // still moves, so the reader can see that the log kept running.
          pendingRef.current = [msg, ...pendingRef.current].slice(0, 500)
          setPendingCount(pendingRef.current.length)
          return
        }

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

  const [pendingCount, setPendingCount] = useState(0)

  // Resuming lets the held-back lines in. The view goes back to the newest
  // line, because that is where the reader left the log.
  const resume = () => {
    const held = pendingRef.current
    pendingRef.current = []
    setPendingCount(0)
    setPaused(false)
    pausedRef.current = false

    if (held.length > 0) {
      setLines(prev => [...held, ...prev].slice(0, 500))
    }

    stickToTop.current = true
    boxRef.current?.scrollTo(0, 0)
  }

  const pause = () => {
    setPaused(true)
    pausedRef.current = true
  }

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
          {pendingCount > 0 && (
            <Box component="span" sx={{ ml: 0.5, color: 'warning.main' }}>
              (+{pendingCount})
            </Box>
          )}
        </Typography>
        {/* 暂停滚动：日志跑得比人读得快，一直追着顶部看，什么也读不到。
            暂停后新行不进入列表，列表就不会被顶动，正在读的那一行留在原地。 */}
        <Tooltip title={paused ? i18n.t('resumeLogs') : i18n.t('pauseLogs')}>
          <IconButton
            size="small"
            onClick={() => {
              if (paused) {
                resume()
              } else {
                pause()
              }
            }}
            aria-label={paused ? i18n.t('resumeLogs') : i18n.t('pauseLogs')}
          >
            {paused
              ? <PlayArrowIcon fontSize="small" />
              : <PauseIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
        <Tooltip title={i18n.t('close')}>
          <IconButton size="small" onClick={onClose}>
            <CloseIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Box
        ref={boxRef}
        onScroll={e => {
          // Keep following only while the reader is at the newest end. While
          // the panel is paused the reader is looking at something else on
          // purpose, so the position is not a wish to follow.
          if (!pausedRef.current) {
            stickToTop.current = e.currentTarget.scrollTop <= 2
          }
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
