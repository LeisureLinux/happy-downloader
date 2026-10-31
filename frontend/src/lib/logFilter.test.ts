import { describe, expect, it } from 'vitest'
import { matches, parseLogLine } from './logFilter'

const info = 'time=2026-10-31T20:28:12Z level=INFO msg="download worker spawned" worker=1'
const request = 'time=2026-10-31T20:28:11Z level=INFO msg=request client=127.0.0.1 method=POST path=/rpc/http status=200'
const warn = 'time=2026-10-31T20:28:12Z level=WARN msg="no window available"'
const error = 'time=2026-10-31T20:28:12Z level=ERROR msg="failed to start download" err="exit status 1"'

describe('parseLogLine', () => {
  it('reads the level out of the text', () => {
    expect(parseLogLine(info).level).toBe('info')
    expect(parseLogLine(warn).level).toBe('warn')
    expect(parseLogLine(error).level).toBe('error')
  })

  it('treats a line with no level as information', () => {
    // The lines written before slog's format was in use have no level, and
    // hiding them would hide the history the panel opens with.
    const old = '2026/10/31 20:28:12 starting server host=0.0.0.0'
    expect(parseLogLine(old).level).toBe('info')
    expect(parseLogLine(old).request).toBe(false)
  })

  it('recognises a request line', () => {
    expect(parseLogLine(request).request).toBe(true)
    expect(parseLogLine(info).request).toBe(false)
  })
})

describe('matches', () => {
  it('keeps everything by default', () => {
    for (const line of [info, request, warn, error]) {
      expect(matches(parseLogLine(line), 'all')).toBe(true)
    }
  })

  it('hides only the request lines when asked to', () => {
    expect(matches(parseLogLine(request), 'requests')).toBe(false)
    expect(matches(parseLogLine(info), 'requests')).toBe(true)
    expect(matches(parseLogLine(warn), 'requests')).toBe(true)
  })

  it('keeps warnings and errors together', () => {
    expect(matches(parseLogLine(info), 'warn')).toBe(false)
    expect(matches(parseLogLine(warn), 'warn')).toBe(true)
    expect(matches(parseLogLine(error), 'warn')).toBe(true)
  })

  it('keeps only errors', () => {
    expect(matches(parseLogLine(warn), 'error')).toBe(false)
    expect(matches(parseLogLine(error), 'error')).toBe(true)
  })
})
