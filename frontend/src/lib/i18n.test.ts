import { describe, expect, it } from 'vitest'

// Translation parity.
//
// A missing key does not fail loudly at runtime — it falls back to English, so
// a language can be half-translated for months without anyone noticing. This
// is the check that was missing when thirteen of the fifteen languages turned
// out to be short by about eighty keys each.
//
// The files are loaded through vite's yaml plugin, the same path the app uses,
// so a file that parses here is a file that loads in the browser.
const modules = import.meta.glob<{ default: { keys: Record<string, unknown> } }>(
  '../assets/i18n/*.yaml',
  { eager: true }
)

const codeOf = (path: string) => path.split('/').pop()!.replace(/\.yaml$/, '')

const files = Object.fromEntries(
  Object.entries(modules).map(([path, mod]) => [codeOf(path), mod.default.keys])
)

const base = Object.keys(files.en_US)

describe('translations', () => {
  it('has a reference file', () => {
    expect(base.length).toBeGreaterThan(0)
  })

  it('every language carries exactly the reference keys', () => {
    const problems: string[] = []

    for (const [code, keys] of Object.entries(files)) {
      const missing = base.filter(k => !(k in keys))
      const extra = Object.keys(keys).filter(k => !base.includes(k))

      if (missing.length) problems.push(`${code}: missing ${missing.join(', ')}`)
      if (extra.length) problems.push(`${code}: extra ${extra.join(', ')}`)
    }

    expect(problems).toEqual([])
  })

  it('keeps the folder labels apart from each other', () => {
    // The labels for the folder actions are four different things, and they
    // sit next to each other in the file: make a folder, move files into one,
    // rename one, delete one.
    //
    // They were confused once — the move button's label became the rename
    // button's label, and the delete tooltip became the explanation of why a
    // full folder cannot be deleted. The parity test above cannot see that:
    // every key exists, and every value is a string in the right language.
    // What catches it is insisting that they are different.
    const en = files.en_US

    const labels = [
      'folderCreateButton',
      'folderMoveButton',
      'folderRenameButton',
    ]

    for (const key of labels) {
      for (const other of labels) {
        if (key === other) continue
        expect(en[key]).not.toBe(en[other])
      }
    }

    // The delete tooltip is about deleting; the other one is about why it
    // cannot be done.
    expect(en.folderDelete).not.toBe(en.folderNotEmpty)
  })

  it('no language is left as the English string', () => {
    // Words that are genuinely the same in both — Proxy, Cookies, Host — would
    // be false positives, so only multi-word values are considered. A whole
    // sentence still in English is a translation nobody finished.
    const sameAsEnglish: string[] = []

    for (const [code, keys] of Object.entries(files)) {
      if (code === 'en_US') continue

      for (const [key, value] of Object.entries(keys)) {
        if (typeof value !== 'string' || value !== files.en_US[key]) continue
        // Count only tokens that contain letters. "Video + audio" is two
        // words, not three, and it is correct Italian as written.
        const words = value.trim().split(/\s+/).filter(t => /\p{L}/u.test(t))
        if (words.length < 3) continue
        sameAsEnglish.push(`${code}.${key}: ${value}`)
      }
    }

    expect(sameAsEnglish).toEqual([])
  })

  it('every language has a translated product name', () => {
    for (const [code, keys] of Object.entries(files)) {
      expect(keys.appName, code).toBeTypeOf('string')
    }
  })
})
