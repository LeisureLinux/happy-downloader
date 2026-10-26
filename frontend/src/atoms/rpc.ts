import { atom } from 'jotai'
import { RPCClient } from '../lib/rpcClient'
import { rpcHTTPEndpoint, rpcWebSocketEndpoint } from './settings'

export const rpcClientState = atom((get) =>
  new RPCClient(
    get(rpcHTTPEndpoint),
    get(rpcWebSocketEndpoint)
  ),
)
