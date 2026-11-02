import {ApplyTheme, SystemTheme} from '../wailsjs/go/main/App'

export type ThemePref = 'auto' | 'light' | 'dark'

// Системную тему спрашиваем у бэкенда (XDG-портал на Linux, реестр на Windows): WebKitGTK
// не всегда отражает её в prefers-color-scheme. Если бэкенд не знает — берём prefers-color-scheme.
const media = window.matchMedia('(prefers-color-scheme: dark)')
const POLL_MS = 3000

let pref: ThemePref = 'auto'
let timer: number | undefined
let applied = ''

async function resolve(): Promise<'light' | 'dark'> {
  if (pref !== 'auto') return pref
  try {
    const sys = await SystemTheme()
    if (sys === 'light' || sys === 'dark') return sys
  } catch {
    // нет связи с бэкендом — ниже fallback
  }
  return media.matches ? 'dark' : 'light'
}

async function apply() {
  const theme = await resolve()
  if (theme === applied) return
  applied = theme
  document.documentElement.dataset.theme = theme
  ApplyTheme(theme === 'dark').catch(() => {})
}

// setTheme применяет выбор пользователя; в режиме auto следит за системной темой.
export function setTheme(p: ThemePref) {
  pref = p
  clearInterval(timer)
  media.onchange = null
  if (p === 'auto') {
    timer = window.setInterval(apply, POLL_MS)
    media.onchange = apply
  }
  apply()
}
