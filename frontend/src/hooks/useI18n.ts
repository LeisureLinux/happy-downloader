import { useAtomValue } from 'jotai'
import { languageState } from '../atoms/settings'
import Translator from '../lib/i18n'

// Subscribing to the language atom is what makes a language switch visible.
// The translator itself is a plain singleton — reading it without a
// subscription meant a component kept its old strings until some unrelated
// state change happened to re-render it.
export const useI18n = () => {
  useAtomValue(languageState)

  const instance = Translator.instance

  return {
    i18n: instance,
    t: instance.t.bind(instance)
  }
}
