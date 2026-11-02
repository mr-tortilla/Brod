const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']

export function formatBytes(n: number): string {
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${i === 0 ? n : n.toFixed(n < 10 ? 1 : 0)} ${units[i]}`
}

export function formatDuration(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000))
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}`
}

const regionNames = (() => {
  try {
    return new Intl.DisplayNames(['ru'], {type: 'region'})
  } catch {
    return null
  }
})()

export function countryName(code: string): string {
  if (!code) return ''
  try {
    return regionNames?.of(code.toUpperCase()) ?? code
  } catch {
    return code
  }
}

export function flag(code: string): string {
  if (!/^[a-z]{2}$/i.test(code)) return ''
  return String.fromCodePoint(...[...code.toUpperCase()].map(c => 0x1f1a5 + c.charCodeAt(0)))
}

// errText превращает ошибку из Go-вызова в текст: Wails отдаёт то строку, то Error.
export function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
