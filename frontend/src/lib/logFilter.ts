// What the log panel shows, out of everything the server writes.
//
// The server logs one line per HTTP request. The interface polls, so those lines
// arrive every couple of seconds and push everything else off the top — the
// panel is unreadable for the reason anybody opens it, which is that something
// went wrong.
//
// The level is in the text of the line, because the log is plain text: slog's
// format puts `level=INFO` in it, and the older lines from before the format
// changed have no level at all. Both are read here rather than by changing what
// the server writes, so the filter also works on a deployment that is older
// than this build.

export type LogCategory = 'all' | 'requests' | 'warn' | 'error'

export const logCategories: LogCategory[] = ['all', 'requests', 'warn', 'error']

const levelPattern = /level=(DEBUG|INFO|WARN|ERROR)/

// A request line is the server reporting an HTTP call it served. It is not a
// failure and it is not what somebody opened the panel for, but it is the only
// category that can be hidden without hiding something that might matter.
const requestPattern = /msg=request\b/

export type LogLine = {
  text: string
  level: 'debug' | 'info' | 'warn' | 'error'
  request: boolean
}

export const parseLogLine = (text: string): LogLine => {
  const levelMatch = levelPattern.exec(text)
  const level = (levelMatch?.[1]?.toLowerCase() ?? 'info') as LogLine['level']

  return { text, level, request: requestPattern.test(text) }
}

// matches says whether a line belongs in the selected category.
export const matches = (line: LogLine, category: LogCategory): boolean => {
  switch (category) {
    case 'requests':
      // Everything except the request lines.
      return !line.request
    case 'warn':
      return line.level === 'warn' || line.level === 'error'
    case 'error':
      return line.level === 'error'
    case 'all':
    default:
      return true
  }
}
