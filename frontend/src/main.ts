import {createApp} from 'vue'
import {polyfillCountryFlagEmojis} from 'country-flag-emoji-polyfill'
import flagsFont from 'country-flag-emoji-polyfill/dist/TwemojiCountryFlags.woff2?url'
import App from './App.vue'
import './style.css';

// Windows намеренно не рисует эмодзи-флаги (вместо 🇩🇪 — буквы DE). Полифил подключает шрифт
// с флагами только там, где их нет, и только для символов флагов. Шрифт встроен в сборку,
// без обращения к CDN.
polyfillCountryFlagEmojis('Twemoji Country Flags', flagsFont)

createApp(App).mount('#app')
