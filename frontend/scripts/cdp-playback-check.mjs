// Check what the player says when the browser cannot decode the file.
//
// The AV1 decoder is stubbed out, which is what an iPhone 12 is: no AV1
// hardware decoder, and Safari has no software one either.
//
//   node scripts/cdp-playback-check.mjs <url>

const url = process.argv[2]

// The page is opened blank, not at the target: the script that stubs the
// decoder has to be installed before the document is created, and a page that
// has already loaded will not run it.
const list = await (await fetch('http://127.0.0.1:9222/json/new?about:blank', {
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
    const target = rows.find(r => r.innerText.includes('Rick Astley'))
    if (!target) return { error: 'row not found' }
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
    const alert = dlg.querySelector('.MuiAlert-message')
    const link = dlg.querySelector('a[download]')
    return {
      hasVideo: !!dlg.querySelector('video'),
      title: dlg.querySelector('.MuiAlert-title')?.innerText
        ?? dlg.querySelector('.MuiAlert-message')?.firstElementChild?.innerText ?? null,
      body: alert?.innerText ?? null,
      downloadButton: link ? link.innerText : null,
      downloadHref: link ? link.getAttribute('href')?.slice(0, 60) : null,
    }
  })()`,
  returnByValue: true,
})

console.log('step2:', JSON.stringify(after.result.value, null, 2))

// A picture of the message, which is the only way to see whether it reads
// well rather than merely says the right words.
const shot = await send('Page.captureScreenshot', { format: 'png' })
const { writeFileSync } = await import('node:fs')
writeFileSync('/tmp/hd-playback-msg.png', Buffer.from(shot.data, 'base64'))
console.log('screenshot: /tmp/hd-playback-msg.png')

await send('Page.close').catch(() => {})
ws.close()
process.exit(0)
