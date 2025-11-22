import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'

import en from './locales/en.json'
import zh from './locales/zh.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import es from './locales/es.json'
import fr from './locales/fr.json'
import de from './locales/de.json'
import hi from './locales/hi.json'
import ar from './locales/ar.json'
import bn from './locales/bn.json'
import pt from './locales/pt.json'
import ru from './locales/ru.json'
import id from './locales/id.json'

i18n
  .use(LanguageDetector) // Detect user language
  .use(initReactI18next) // Pass i18n to react-i18next
  .init({
    resources: {
      en: { translation: en },
      zh: { translation: zh },
      ja: { translation: ja },
      ko: { translation: ko },
      es: { translation: es },
      fr: { translation: fr },
      de: { translation: de },
      hi: { translation: hi },
      ar: { translation: ar },
      bn: { translation: bn },
      pt: { translation: pt },
      ru: { translation: ru },
      id: { translation: id },
    },
    fallbackLng: 'en',
    supportedLngs: ['en', 'zh', 'ja', 'ko', 'es', 'fr', 'de', 'hi', 'ar', 'bn', 'pt', 'ru', 'id'],
    debug: false,
    interpolation: {
      escapeValue: false, // React already escapes
    },
  })

export default i18n
