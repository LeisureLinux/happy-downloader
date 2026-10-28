// A tiny CDP driver: evaluate JavaScript in a page and print the result.
//
// Used to verify the caption-downloading label in the file browser without a
// human in the loop. Node 22 has a WebSocket client built in, so this needs
// no dependency.
//
//   node scripts/cdp-eval.mjs <url> <script-file> [--lang chinese] [--wait 3000]

import { readFileSync } from 'node:fs'

const [url, scriptFile, ...rest] = process.argv.slice(2)

const langArg = rest.indexOf('--lang')
const waitArg = rest.indexOf('--wait')
const lang = langArg >= 0 ? rest[langArg + 1] : null
const wait = waitArg >= 0 ? Number(rest[waitArg + 1]) : 3000

const list = await (await fetch('http://127.0.0.1:9222/json/new?' + encodeURIComponent(url), {
  method: 'PUT',
})).json()

const ws = new WebSocket(list.webSocketDebuggerUrl)

let id = 0
const pending = new Map()

const send = (method, params = {}) => new Promise((resolve, reject) => {
  const msgId = ++id
  pending.set(msgId, { resolve, reject })
  ws.send(JSON.stringify({ id: msgId, method, params }))
})

ws.addEventListener('message', ev => {
  const msg = JSON.parse(ev.data)
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id)
    pending.delete(msg.id)
    msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result)
  }
})

await new Promise(r => ws.addEventListener('open', r))

await send('Runtime.enable')

if (lang) {
  await send('Runtime.evaluate', {
    expression: `localStorage.setItem('language', JSON.stringify(${JSON.stringify(lang)}));`,
  })
  await send('Page.enable')
  await send('Page.reload')
  await new Promise(r => setTimeout(r, wait))
}

const result = await send('Runtime.evaluate', {
  expression: readFileSync(scriptFile, 'utf8'),
  awaitPromise: true,
  returnByValue: true,
})

if (result.exceptionDetails) {
  console.error(JSON.stringify(result.exceptionDetails, null, 2))
  process.exit(1)
}

console.log(JSON.stringify(result.result.value, null, 2))

// A picture, when the caller asked for one. The words can be right and the
// layout still be wrong.
if (process.env.SHOT) {
  const shot = await send('Page.captureScreenshot', { format: 'png' })
  const { writeFileSync } = await import('node:fs')
  writeFileSync(process.env.SHOT, Buffer.from(shot.data, 'base64'))
  console.log('screenshot:', process.env.SHOT)
}

await send('Page.close').catch(() => {})
ws.close()
process.exit(0)
