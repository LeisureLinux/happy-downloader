//@ts-ignore
import i18n from '../assets/i18n.yaml'
//@ts-ignore
import fallback from '../assets/i18n/en_US.yaml'

// Every translation file is bundled eagerly, and the lookup is synchronous.
//
// It used to be a dynamic import: setLanguage() returned before the file had
// loaded, so the first render after a reload drew English from the fallback,
// and when the import finally resolved nothing re-rendered. The visible effect
// was a language that applied only once some unrelated state change happened
// to re-render the tree — the title could be Chinese while the headings were
// still English.
//
// The files total well under a hundred kilobytes and this is a LAN app; a
// language that is always correct is worth more than a lazy chunk.
const modules = import.meta.glob<{ default: { keys: Record<string, string> } }>(
  '../assets/i18n/*.yaml',
  { eager: true }
)

function loadKeys(file: string): Record<string, string> | null {
  const wanted = `../assets/i18n/${file.replace(/\.yaml$/, '')}.yaml`
  const found = modules[wanted]
  return found?.default?.keys ?? null
}

export default class Translator {
  static #instance: Translator

  private language: string
  private current: Record<string, string> = {}

  constructor() {
    this.language = localStorage.getItem('language')?.replaceAll('"', '') ?? 'english'
    this.setLanguage(this.language)
  }

  getLanguage(): string {
    return this.language
  }

  setLanguage(language: string): void {
    this.language = language

    const isoCodeFile: string | undefined = i18n.languages[language]
    if (!isoCodeFile) {
      return
    }

    this.current = loadKeys(isoCodeFile) ?? {}
  }

  t(key: string): string {
    return this.current[key] ?? fallback.keys[key] ?? key
  }

  // For values that are a map rather than a string, like the accent colour
  // names. A dotted key would work, but it hides that these are a group and
  // makes a typo look like a missing key instead of a bug.
  tSub(group: string, key: string): string {
    const g = this.current[group] ?? fallback.keys[group]

    if (g && typeof g === 'object') {
      const hit = (g as Record<string, unknown>)[key]
      if (typeof hit === 'string') {
        return hit
      }
    }

    return key
  }

  public static get instance(): Translator {
    if (!Translator.#instance) {
      Translator.#instance = new Translator()
    }

    return Translator.#instance
  }
}
