import { Observable } from 'rxjs'
import type { DLMetadata, LiveStreamProgress, RPCRequest, RPCResponse, RPCResult } from '../types'

import { WebSocketSubject, webSocket } from 'rxjs/webSocket'

type DownloadRequestArgs = {
  url: string,
  // The arguments as a list, not a joined string.
  //
  // They used to be a string that this class parsed back into a list, and the
  // parser looked for "-o" with indexOf and replace. That finds the "o" inside
  // "--merge-output-format", so the flag arrived at the server as "--merge"
  // and the format "mp4" as its value — a combination the server's argument
  // whitelist rejects, which left the download stuck at "preparing" with
  // nothing in the log to explain it. A list needs no parsing.
  args: string[],
  pathOverride?: string,
  renameTo?: string,
  playlist?: boolean
}

export class RPCClient {
  private seq: number
  private httpEndpoint: string
  private readonly _socket$: WebSocketSubject<any>

  constructor(httpEndpoint: string, webSocketEndpoint: string) {
    this.seq = 0
    this.httpEndpoint = httpEndpoint
    this._socket$ = webSocket<any>({ url: webSocketEndpoint })
  }

  public get socket$(): Observable<RPCResponse<RPCResult[]>> {
    return this._socket$
  }

  private incrementSeq() {
    return String(this.seq++)
  }

  private send(req: RPCRequest) {
    this._socket$.next({
      ...req,
      id: this.incrementSeq(),
    })
  }

  private async sendHTTP<T>(req: RPCRequest) {
    const res = await fetch(this.httpEndpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        ...req,
        id: this.incrementSeq(),
      })
    })
    const data: RPCResponse<T> = await res.json()

    // An RPC error means the server refused the request. Throwing it here
    // makes it visible where the request was made — the download form shows it
    // as a toast — instead of the request appearing to succeed and the
    // download never appearing in the queue.
    //
    // The server speaks net/rpc/jsonrpc, whose error is an object with a
    // message; the number in the type was wrong.
    if (data.error) {
      const message = typeof data.error === 'object' && data.error !== null
        ? data.error.message
        : String(data.error)

      throw new Error(message || 'RPC error')
    }

    return data
  }

  public download(req: DownloadRequestArgs) {
    if (!req.url) {
      return
    }

    // An explicit output filename is the user's choice, not an argument, so it
    // travels in its own field rather than inside the argument list.
    const sanitizedArgs = req.args.filter(a => a !== '-o' && a !== '--output')

    if (req.playlist) {
      return this.sendHTTP({
        method: 'Service.ExecPlaylist',
        params: [{
          URL: req.url,
          Params: sanitizedArgs,
          Path: req.pathOverride,
          Rename: req.renameTo ?? '',
        }]
      })
    }
    this.sendHTTP({
      method: 'Service.Exec',
      params: [{
        URL: req.url.split('?list')[0]!,
        Params: sanitizedArgs,
        Path: req.pathOverride,
        Rename: req.renameTo ?? '',
      }]
    })
  }

  public formats(url: string) {
    if (url) {
      return this.sendHTTP<DLMetadata>({
        method: 'Service.Formats',
        params: [{
          URL: url.split('?list')[0]!,
        }]
      })
    }
  }

  public running() {
    this.send({
      method: 'Service.Running',
      params: [],
    })
  }

  public kill(id: string) {
    return this.sendHTTP({
      method: 'Service.Kill',
      params: [id],
    })
  }

  // Stop a running download but keep it in the queue, so it can be resumed.
  public pause(id: string) {
    return this.sendHTTP({
      method: 'Service.Pause',
      params: [id],
    })
  }

  // Start a paused download again, with the arguments it was created with.
  public resume(id: string) {
    return this.sendHTTP({
      method: 'Service.Resume',
      params: [id],
    })
  }

  public clear(id: string) {
    return this.sendHTTP({
      method: 'Service.Clear',
      params: [id],
    })
  }

  // Re-queue a failed download with the arguments it was created with.
  public retry(id: string) {
    return this.sendHTTP({
      method: 'Service.Retry',
      params: [id],
    })
  }

  public killAll() {
    return this.sendHTTP({
      method: 'Service.KillAll',
      params: [],
    })
  }

  public freeSpace() {
    return this.sendHTTP<number>({
      method: 'Service.FreeSpace',
      params: [],
    })
  }

  public directoryTree() {
    return this.sendHTTP<string[]>({
      method: 'Service.DirectoryTree',
      params: [],
    })
  }

  public execLivestream(url: string) {
    return this.sendHTTP({
      method: 'Service.ExecLivestream',
      params: [{
        URL: url
      }]
    })
  }

  public progressLivestream() {
    return this.sendHTTP<LiveStreamProgress>({
      method: 'Service.ProgressLivestream',
      params: []
    })
  }

  public killLivestream(url: string) {
    return this.sendHTTP({
      method: 'Service.KillLivestream',
      params: [url]
    })
  }

  public killAllLivestream() {
    return this.sendHTTP({
      method: 'Service.KillAllLivestream',
      params: []
    })
  }

  // Whether a newer yt-dlp is available. The server caches the answer, so
  // this is cheap enough to call when the page loads.
  public checkYtdlpUpdate() {
    return this.sendHTTP<{
      current: string
      latest?: string
      outdated: boolean
      error?: string
    }>({
      method: 'Service.CheckYtdlpUpdate',
      params: [],
    })
  }

  // Install a newer yt-dlp.
  public updateYtdlp() {
    return this.sendHTTP({
      method: 'Service.UpdateYtdlp',
      params: [],
    })
  }

  public updateExecutable() {
    return this.sendHTTP({
      method: 'Service.UpdateExecutable',
      params: []
    })
  }

  // yt-dlp reports its own version; used to tell an upgrade from a no-op.
  public version() {
    return this.sendHTTP({
      method: 'Service.GetVersion',
      params: []
    })
  }

  public clearCompleted() {
    return this.sendHTTP({
      method: 'Service.ClearCompleted',
      params: []
    })
  }
}