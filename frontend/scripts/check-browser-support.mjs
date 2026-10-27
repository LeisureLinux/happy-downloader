#!/usr/bin/env node
// Fail the build when the bundle needs a browser newer than Windows 7 has.
//
// The last Chrome that installs on Windows 7 is 109, and it is the browser this
// is used from. Two things can make a build require something newer:
//
//   - syntax, which vite's build.target handles (see vite.config.mts)
//   - an API that does not exist yet, which no amount of downlevelling fixes
//
// Only the second is checked here, because only the second can slip through.
// `Array.prototype.toSorted` shipped in Chrome 110 and was sitting in the
// settings panel; on 109 it is a TypeError with no hint about what called it.
//
// Usage: node scripts/check-browser-support.mjs [dist-dir]

import { readdirSync, readFileSync, existsSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const CHROME_FLOOR = 109

const dir = resolve(
  process.argv[2] ?? join(dirname(fileURLToPath(import.meta.url)), '..', 'dist')
)

// Each entry is [pattern, first Chrome version that has it, what it is]. The
// version is only used to say why it is wrong, so a wrong number here is a
// confusing message rather than a missed check.
const JS = [
  [/\.toSorted\(/, 110, 'Array.prototype.toSorted'],
  [/\.toReversed\(/, 110, 'Array.prototype.toReversed'],
  [/\.toSpliced\(/, 110, 'Array.prototype.toSpliced'],
  [/\.with\(/, 110, 'Array.prototype.with'],
  [/Object\.groupBy\(/, 117, 'Object.groupBy'],
  [/Map\.groupBy\(/, 117, 'Map.groupBy'],
  [/Promise\.withResolvers\(/, 119, 'Promise.withResolvers'],
  [/Promise\.try\(/, 128, 'Promise.try'],
  [/Array\.fromAsync\(/, 121, 'Array.fromAsync'],
  [/\.isWellFormed\(/, 111, 'String.prototype.isWellFormed'],
  [/\.toWellFormed\(/, 111, 'String.prototype.toWellFormed'],
  [/URL\.canParse\(/, 120, 'URL.canParse'],
  [/URL\.parse\(/, 126, 'URL.parse'],
  [/AbortSignal\.any\(/, 116, 'AbortSignal.any'],
  [/\.symmetricDifference\(/, 122, 'Set.prototype.symmetricDifference'],
  [/\.isSubsetOf\(/, 122, 'Set.prototype.isSubsetOf'],
  [/\.isSupersetOf\(/, 122, 'Set.prototype.isSupersetOf'],
  [/\.isDisjointFrom\(/, 122, 'Set.prototype.isDisjointFrom'],
  [/\.union\(/, 122, 'Set.prototype.union'],
  [/\.intersection\(/, 122, 'Set.prototype.intersection'],
  [/document\.startViewTransition\(/, 111, 'document.startViewTransition'],
  [/\.showPopover\(/, 114, 'HTMLElement.showPopover'],
  [/\.hidePopover\(/, 114, 'HTMLElement.hidePopover'],
  [/\.togglePopover\(/, 114, 'HTMLElement.togglePopover'],
  [/scheduler\.yield/, 129, 'scheduler.yield'],
  [/\.moveBefore\(/, 133, 'Element.moveBefore'],
  [/Intl\.DurationFormat/, 129, 'Intl.DurationFormat'],
]

// Feature-detected calls. The call is present in the bundle, but it only runs
// after the library has checked that the function exists, so an old browser
// takes the other branch instead of throwing.
const ALLOWED = new Map([
  [
    'document.startViewTransition',
    'react-router checks typeof document.startViewTransition before calling it',
  ],
])

const CSS = [
  [/color-mix\(/, 111, 'color-mix()'],
  [/oklch\(/, 111, 'oklch()'],
  [/oklab\(/, 111, 'oklab()'],
  [/light-dark\(/, 123, 'light-dark()'],
  [/text-wrap:\s*balance/, 114, 'text-wrap: balance'],
  [/field-sizing/, 123, 'field-sizing'],
  [/@scope/, 118, '@scope'],
  [/@starting-style/, 117, '@starting-style'],
  [/interpolate-size/, 129, 'interpolate-size'],
  [/view-transition-name/, 111, 'view transitions'],
  [/:user-valid/, 119, ':user-valid'],
  [/:user-invalid/, 119, ':user-invalid'],
  [/anchor-name/, 125, 'anchor positioning'],
]

if (!existsSync(dir)) {
  console.error(`no build to check at ${dir} — run the frontend build first`)
  process.exit(2)
}

const assets = join(dir, 'assets')
const files = existsSync(assets)
  ? readdirSync(assets).map(f => join(assets, f))
  : readdirSync(dir).map(f => join(dir, f))

const problems = []

for (const file of files) {
  const isJS = file.endsWith('.js')
  const isCSS = file.endsWith('.css')
  if (!isJS && !isCSS) continue

  const code = readFileSync(file, 'utf8')
  for (const [pattern, version, name] of isJS ? JS : CSS) {
    // The whole bundle is minified onto one line, so the line number is not
    // useful; a snippet around the match is.
    const match = pattern.exec(code)
    if (match && version > CHROME_FLOOR && !ALLOWED.has(name)) {
      const at = Math.max(0, match.index - 40)
      problems.push({
        file,
        name,
        version,
        snippet: code.slice(at, at + 100).replace(/\s+/g, ' '),
      })
    }
  }
}

if (problems.length === 0) {
  for (const [name, why] of ALLOWED) {
    console.log(`allowed: ${name} — ${why}`)
  }
  console.log(`ok: nothing in the bundle needs a Chrome newer than ${CHROME_FLOOR}`)
  process.exit(0)
}

for (const p of problems) {
  console.error(
    `${p.file}: uses ${p.name}, which needs Chrome ${p.version} ` +
      `(the floor is ${CHROME_FLOOR})`
  )
  console.error(`  …${p.snippet}…`)
}
console.error(
  '\nReplace it with something that exists in ' + CHROME_FLOOR + ', or stop using it.'
)
process.exit(1)
