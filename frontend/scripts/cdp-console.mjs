// Read the console messages a page produced.
//
// The React error boundary shows "Minified React error #482" and nothing
// else, which is the least useful thing a page can say. The console has the
// real message.
//
//   node scripts/cdp-console.mjs <url>

const url = process.argv[2]

const list = await (await fetch('http://127.0.0.1:9222/json/new?about:blank', {
  method: 'PUT',
})).json()

const ws = new WebSocket(list.webSocketDebuggerUrl)

let id = 0
const pending = new Map()
const messages = []

const send = (method, params = {}) => new Promise((resolve, reject) => {
  const msgId = ++id
  pending.set(msgId, { resolve, reject })
  ws.send(JSON.stringify({ id: msgId, method, params }))
})

ws.addEventListener('message', ev => {
  const msg = JSON.parse(ev.data)

  if (msg.method === 'Runtime.consoleAPICalled' || msg.method === 'Runtime.exceptionThrown') {
    messages.push(msg)
  }

  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id)
    pending.delete(msg.id)
    msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result)
  }
})

await new Promise(r => ws.addEventListener('open', r))

await send('Runtime.enable')
await send('Log.enable')
await send('Page.enable')

await send('Page.navigate', { url })
await new Promise(r => setTimeout(r, 5000))

for (const m of messages) {
  if (m.method === 'Runtime.exceptionThrown') {
    const d = m.params.exceptionDetails
    console.log('EXCEPTION:', d.text)
    if (d.exception?.description) {
      console.log('  ', d.exception.description.slice(0, 600))
    }
    if (d.stackTrace?.callFrames?.length) {
      console.log('  stack:', d.stackTrace.callFrames.slice(0, 4)
        .map(f => `${f.url.split('/').pop()}:${f.lineNumber}`).join(' '))
    }
  } else {
    const args = (m.params.args ?? [])
      .map(a => a.value ?? a.description ?? a.type).join(' ')
    console.log('CONSOLE:', m.params.type, args.slice(0, 500))
  }
}

await send('Page.close').catch(() => {})
ws.close()
process.exit(0)
