// Check what the player does with a file the browser cannot decode.
//
// The browser is asked to pretend it has no AV1 decoder — which is what an
// iPhone 12 is — by stubbing canPlayType before the page loads. The player
// should then show the message instead of a black rectangle.
//
//   node scripts/cdp-playback-check.mjs <url>

const url = process.argv[2]

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

await send('Page.enable')
await send('Runtime.enable')

// Pretend the machine has no AV1 decoder. Every canPlayType that mentions
// AV1 answers "no", which is exactly what an iPhone 12 does.
await send('Page.addScriptToEvaluateOnNewDocument', {
  source: `
    const real = HTMLMediaElement.prototype.canPlayType
    HTMLMediaElement.prototype.canPlayType = function (type) {
      if (/av01/i.test(type)) return ''
      return real.call(this, type)
    }
  `,
})

await send('Page.navigate', { url })
await new Promise(r => setTimeout(r, 4000))

const result = await send('Runtime.evaluate', {
  expression: `(() => {
    const rows = [...document.querySelectorAll('tbody tr')]
    const target = rows.find(r => r.innerText.includes('fridge situation'))
    if (!target) return { error: 'row not found', rows: rows.map(r => r.innerText.slice(0, 30)) }
    const buttons = [...target.querySelectorAll('button')]
    buttons[0].click()
    return { clicked: true }
  })()`,
  returnByValue: true,
})

console.log('step1:', JSON.stringify(result.result.value))

await new Promise(r => setTimeout(r, 2000))

const after = await send('Runtime.evaluate', {
  expression: `(() => {
    const dlg = document.querySelector('[role=dialog]')
    if (!dlg) return { error: 'no dialog' }
    const video = dlg.querySelector('video')
    const alert = dlg.querySelector('.MuiAlert-message')
    return {
      hasVideo: !!video,
      alertText: alert ? alert.innerText : null,
      alertSeverity: dlg.querySelector('.MuiAlert-root')?.getAttribute('class') || null,
      dialogText: dlg.innerText.slice(0, 400),
    }
  })()`,
  returnByValue: true,
})

console.log('step2:', JSON.stringify(after.result.value, null, 2))

await send('Page.close').catch(() => {})
ws.close()
process.exit(0)
