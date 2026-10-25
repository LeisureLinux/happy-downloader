import { useEffect, useMemo, useRef, useState } from 'react'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useAtomValue } from 'jotai'

const token = localStorage.getItem('token')

const LogTerminal: React.FC = () => {
  const [logBuffer, setLogBuffer] = useState<string[]>([])
  const [isConnecting, setIsConnecting] = useState(true)

  const boxRef = useRef<HTMLDivElement>(null)
  // Newest-first rendering: the list is reversed, so "following the tail"
  // means staying pinned to the top of the scroll box rather than the bottom.
  const stickToTopRef = useRef(true)

  const serverAddr = useAtomValue(serverURL)

  const { i18n } = useI18n()

  const eventSource = useMemo(
    () => new EventSource(`${serverAddr}/log/sse?token=${token}`),
    [serverAddr]
  )

  useEffect(() => {
    eventSource.addEventListener('log', event => {
      const msg: string = JSON.parse(event.data)
      setLogBuffer(buff => [msg, ...buff].slice(0, 500))

      // Only yank the view back to the newest entry while the user has not
      // scrolled away to read older lines.
      if (stickToTopRef.current) {
        boxRef.current?.scrollTo(0, 0)
      }
    })

    // TODO: in dev mode it breaks sse
    return () => eventSource.close()
  }, [eventSource])

  useEffect(() => {
    eventSource.onopen = () => setIsConnecting(false)
  }, [eventSource])

  const logEntryStyle = (data: string) => {
    const sx = {}

    if (data.includes("level=ERROR")) {
      return { ...sx, color: 'red' }
    }
    if (data.includes("level=WARN")) {
      return { ...sx, color: 'orange' }
    }

    return sx
  }

  return (

    <div
      ref={boxRef}
      style={{
        fontFamily: 'Roboto Mono',
        height: '70.5vh',
        overflowY: 'auto',
        overflowX: 'auto',
        fontSize: '13.5px',
        fontWeight: '600',
        backgroundColor: 'black',
        color: 'white',
        padding: '0.5rem',
        borderRadius: '0.25rem'
      }}
      onScroll={(e) => {
        // Small tolerance so sub-pixel/zoom rounding does not flap the flag.
        stickToTopRef.current = e.currentTarget.scrollTop <= 2
      }}
    >
      {isConnecting ? <div>{'Connecting...'}</div> : <div>{'Connected!'}</div>}

      {logBuffer.length === 0 && <div>{i18n.t('awaitingLogs')}</div>}

      {logBuffer.map((log, idx) => (
        <div key={idx} style={logEntryStyle(log)}>
          {log}
        </div>
      ))}
    </div>

  )
}

export default LogTerminal