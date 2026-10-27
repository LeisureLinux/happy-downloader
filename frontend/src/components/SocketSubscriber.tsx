import * as O from 'fp-ts/Option'
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { EMPTY, Observable, catchError, exhaustMap, take, timer } from 'rxjs'
import { downloadsState } from '../atoms/downloads'
import { serverAddressAndPortState } from '../atoms/settings'
import { connectedState } from '../atoms/status'
import { useSubscription } from '../hooks/observable'
import { useToast } from '../hooks/toast'
import { useI18n } from '../hooks/useI18n'
import { useRPC } from '../hooks/useRPC'
import { datetimeCompareFunc, isRPCResponse } from '../utils'
import { useAtom, useAtomValue, useSetAtom } from 'jotai'

interface Props extends React.HTMLAttributes<HTMLBaseElement> { }

// How often the running-download list is refreshed, over the existing
// WebSocket. Two seconds keeps progress smooth without a request storm. The
// value is fixed rather than configurable: it is not a choice a user would
// make differently, and the slider it replaced only offered a trade nobody
// needed to weigh.
const STATUS_POLL_MS = 2000

// How long to wait between attempts to re-establish a dropped WebSocket.
const RECONNECT_MS = 2000

const SocketSubscriber: React.FC<Props> = () => {
  const [connected, setIsConnected] = useAtom(connectedState)
  const setDownloads = useSetAtom(downloadsState)

  const serverAddressAndPort = useAtomValue(serverAddressAndPortState)

  const { i18n } = useI18n()
  const { client } = useRPC()
  const { pushMessage } = useToast()

  const navigate = useNavigate()

  // Reconnect loop.
  //
  // The socket used to be probed with take(1): subscribe, wait for the first
  // message, unsubscribe. That looked harmless because WebSocketSubject is
  // multicast, but it closes the socket when the last subscriber leaves — and
  // "last" counted the probe. If the probe's take(1) completed while the state
  // subscription was between mounts, the socket was closed underneath it, and
  // the page reported that the server was unreachable while the server was
  // answering perfectly.
  //
  // The probe now keeps its subscription until it is deliberately torn down,
  // and it is only used to decide whether to retry.
  useEffect(() => {
    if (connected) return

    let probe: { unsubscribe: () => void } | undefined

    const sub = timer(0, RECONNECT_MS).pipe(
      exhaustMap(() => {
        probe?.unsubscribe()

        return new Observable<void>(observer => {
          const inner = client.socket$.subscribe({
            next: () => observer.next(),
            error: () => observer.complete(),
          })

          probe = { unsubscribe: () => inner.unsubscribe() }

          return () => inner.unsubscribe()
        }).pipe(
          take(1),
          catchError(() => EMPTY)
        )
      })
    ).subscribe(() => {
      setIsConnected(true)
      pushMessage(
        `${i18n.t('toastConnected')} (${serverAddressAndPort})`,
        'success'
      )
    })

    return () => {
      sub.unsubscribe()
      probe?.unsubscribe()
    }
  }, [connected, client])

  useSubscription(
    client.socket$,
    event => {
      if (!isRPCResponse(event)) { return }
      if (!Array.isArray(event.result)) { return }

      if (event.result) {
        return setDownloads(
          O.of(event.result
            .filter(f => !!f.info.url).sort((a, b) => datetimeCompareFunc(
              b.info.created_at,
              a.info.created_at,
            )),
          )
        )
      }
      setDownloads(O.none)
    },
    err => {
      console.error(err)
      // Reset the flag before navigating: the reconnect effect keys off it, and
      // leaving it true meant a restart stranded the page on /error forever
      // because nothing ever tried the socket again.
      setIsConnected(false)
      pushMessage(
        `${i18n.t('connectionError')} (${serverAddressAndPort})`,
        "error"
      )
      navigate('/error')
    }
  )

  useEffect(() => {
    if (connected) {
      const sub = timer(0, STATUS_POLL_MS).subscribe(() => client.running())
      return () => sub.unsubscribe()
    }
  }, [connected, client])

  return null
}

export default SocketSubscriber