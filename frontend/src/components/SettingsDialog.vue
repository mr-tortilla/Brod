<script lang="ts" setup>
import {onMounted, ref} from 'vue'
import {DefaultSettings, GetSettings, OpenLogFolder, SaveSettings} from '../../wailsjs/go/main/App'
import {main, store} from '../../wailsjs/go/models'
import {errText} from '../format'
import {setTheme, type ThemePref} from '../theme'

const emit = defineEmits<{
  saved: [result: main.SaveSettingsResult]
  close: []
}>()

type Tab = 'connection' | 'tun' | 'dns' | 'bypass' | 'ui'
const tabs: { id: Tab, label: string }[] = [
  {id: 'connection', label: 'Подключение'},
  {id: 'tun', label: 'TUN'},
  {id: 'dns', label: 'DNS'},
  {id: 'bypass', label: 'Исключения'},
  {id: 'ui', label: 'Прочее'},
]

const tab = ref<Tab>('connection')
const form = ref<store.Settings | null>(null)
const bypassText = ref('')
const savedTheme = ref<ThemePref>('auto') // для отката предпросмотра темы при отмене
const saving = ref(false)
const error = ref('')

const presets: { label: string, items: string[] }[] = [
  {label: 'Локальные сети', items: ['local', 'lan', '192.168.0.0/16', '10.0.0.0/8', '172.16.0.0/12']},
]

onMounted(async () => {
  try {
    const st = await GetSettings()
    form.value = st
    bypassText.value = st.bypass.join('\n')
    savedTheme.value = st.theme as ThemePref
  } catch (e) {
    error.value = errText(e)
  }
})

function lines() {
  return bypassText.value.split('\n').map(s => s.trim()).filter(Boolean)
}

function addPreset(items: string[]) {
  const current = lines()
  bypassText.value = [...current, ...items.filter(i => !current.includes(i))].join('\n')
}

function pickTheme(t: ThemePref) {
  form.value!.theme = t
  setTheme(t) // предпросмотр
}

// Сбрасываются все настройки, кроме списка исключений: его жалко терять.
async function resetDefaults() {
  const def = await DefaultSettings()
  def.bypass = lines()
  form.value = def
  setTheme(def.theme as ThemePref)
}

async function openLogs() {
  try {
    await OpenLogFolder()
  } catch (e) {
    error.value = errText(e)
  }
}

function cancel() {
  setTheme(savedTheme.value)
  emit('close')
}

async function save() {
  if (!form.value) return
  saving.value = true
  error.value = ''
  try {
    const st = store.Settings.createFrom({...form.value, bypass: lines()})
    const res = await SaveSettings(st)
    savedTheme.value = st.theme as ThemePref
    if (res.invalid.length) {
      tab.value = 'bypass'
      error.value = `Не распознано, эти строки игнорируются: ${res.invalid.join(', ')}`
      return
    }
    emit('saved', res)
  } catch (e) {
    error.value = errText(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="overlay" @mousedown.self="cancel">
    <form class="dialog" novalidate @submit.prevent="save">
      <h2>Настройки</h2>

      <div class="tabs">
        <button v-for="t in tabs" :key="t.id" type="button" :class="{active: tab === t.id}" @click="tab = t.id">
          {{ t.label }}
        </button>
      </div>

      <p v-if="!form && !error" class="hint">Загрузка…</p>

      <div v-if="form" class="body">
        <template v-if="tab === 'connection'">
          <label class="field">
            <span>Порт локального прокси</span>
            <input v-model.number="form.mixedPort" type="number" min="0" max="65535"/>
            <small>HTTP и SOCKS5 на одном порту. 0 — выбирать свободный при каждом подключении.</small>
          </label>
          <label class="check">
            <input v-model="form.allowLan" type="checkbox"/>
            <span>
              Разрешить подключения из локальной сети
              <small>Другие устройства смогут ходить через Brod, указав прокси «IP этого компьютера:порт».</small>
            </span>
          </label>
          <label class="check">
            <input v-model="form.bypassLan" type="checkbox"/>
            <span>
              Локальная сеть напрямую
              <small>Роутер, принтеры, NAS и адреса 192.168.x.x, 10.x.x.x доступны без туннеля.</small>
            </span>
          </label>
          <label class="check">
            <input v-model="form.refreshOnConnect" type="checkbox"/>
            <span>
              Обновлять подписку перед подключением
              <small>Подтягивает изменения серверов у провайдера; подключение на секунду дольше.</small>
            </span>
          </label>
          <label class="field">
            <span>Адрес проверки задержки</span>
            <input v-model="form.testUrl" placeholder="http://cp.cloudflare.com/generate_204"/>
            <small>Должен отвечать быстро и без содержимого, обычно это адреса generate_204.</small>
          </label>
        </template>

        <template v-if="tab === 'tun'">
          <label class="field">
            <span>Сетевой стек</span>
            <select v-model="form.tunStack">
              <option value="mixed">mixed — TCP системный, UDP gVisor (рекомендуется)</option>
              <option value="system">system — системный, быстрее</option>
              <option value="gvisor">gvisor — userspace, совместимее</option>
            </select>
          </label>
          <label class="field">
            <span>MTU</span>
            <input v-model.number="form.tunMtu" type="number" min="1280" max="65535"/>
            <small>Если часть сайтов не открывается или зависает, попробуйте 1500 или 1400.</small>
          </label>
          <label class="check">
            <input v-model="form.strictRoute" type="checkbox"/>
            <span>
              Строгая маршрутизация
              <small>Не даёт трафику уйти мимо туннеля. Выключайте, если не работают локальные сервисы.</small>
            </span>
          </label>
          <label class="check">
            <input v-model="form.systemProxyInTun" type="checkbox"/>
            <span>
              Системный прокси и в режиме TUN
              <small>Браузеры передают домен напрямую — исключения по доменам срабатывают точнее,
                а прокси другого VPN-клиента не перехватит трафик.</small>
            </span>
          </label>
        </template>

        <template v-if="tab === 'dns'">
          <label class="field">
            <span>DNS через туннель (DoH)</span>
            <input v-model="form.remoteDns" placeholder="1.1.1.1"/>
            <small>Для всех сайтов, кроме исключений. Запросы шифруются и идут через сервер.</small>
          </label>
          <label class="field">
            <span>DNS напрямую</span>
            <input v-model="form.directDns" placeholder="8.8.8.8"/>
            <small>Для исключений и адресов самих серверов. Запросы идут открыто, мимо туннеля.</small>
          </label>
          <label class="field">
            <span>IPv6</span>
            <select v-model="form.ipv6">
              <option value="off">Выключен — только IPv4 (рекомендуется)</option>
              <option value="prefer_ipv4">Предпочитать IPv4</option>
              <option value="prefer_ipv6">Предпочитать IPv6</option>
            </select>
            <small>Многие серверы не поддерживают IPv6 — тогда сайты открываются с задержкой.</small>
          </label>
        </template>

        <template v-if="tab === 'bypass'">
          <p class="hint">
            Эти сайты открываются напрямую, без туннеля. По одному на строку:
            доменная зона (<code>org</code>, <code>local</code>) — все сайты в ней, сайт со всеми поддоменами
            (<code>example.com</code>), адрес сайта (<code>https://example.com/page</code>),
            IP или подсеть (<code>1.2.3.4</code>, <code>10.0.0.0/8</code>). Строки с <code>#</code> — комментарии.
          </p>
          <div class="presets">
            <button v-for="p in presets" :key="p.label" type="button" @click="addPreset(p.items)">+ {{ p.label }}</button>
          </div>
          <textarea v-model="bypassText" rows="10" placeholder="example.com&#10;local&#10;192.168.0.0/16"/>
        </template>

        <template v-if="tab === 'ui'">
          <div class="field">
            <span>Тема</span>
            <div class="segmented">
              <button type="button" :class="{active: form.theme === 'auto'}" @click="pickTheme('auto')">Как в системе</button>
              <button type="button" :class="{active: form.theme === 'light'}" @click="pickTheme('light')">Светлая</button>
              <button type="button" :class="{active: form.theme === 'dark'}" @click="pickTheme('dark')">Тёмная</button>
            </div>
          </div>
          <div class="field">
            <span>Диагностика</span>
            <div>
              <button type="button" @click="openLogs">Открыть папку с логами</button>
            </div>
            <small>Лог brod.log пригодится, если что-то не работает: в нём ошибки подключения и ядра.</small>
          </div>
        </template>
      </div>

      <p v-if="error" class="error">{{ error }}</p>

      <div class="actions">
        <button type="button" class="reset" :disabled="!form" @click="resetDefaults">Сбросить по умолчанию</button>
        <button type="button" @click="cancel">Отмена</button>
        <button type="submit" class="primary" :disabled="!form || saving">
          {{ saving ? 'Сохранение…' : 'Сохранить' }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

.dialog {
  width: min(600px, calc(100vw - 32px));
  max-height: calc(100vh - 32px);
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

h2 {
  margin: 0;
  font-size: 18px;
}

.tabs, .segmented {
  display: flex;
  gap: 4px;
  background: var(--bg);
  padding: 4px;
  border-radius: var(--radius);
}

.tabs button, .segmented button {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--muted);
  padding: 6px 4px;
}

.tabs button.active, .segmented button.active {
  background: var(--panel-2);
  color: var(--text);
}

.body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow-y: auto;
  min-height: 300px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field > span, .check > span {
  font-size: 13px;
}

small {
  display: block;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.4;
}

.check {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  cursor: pointer;
}

.check input {
  width: auto;
  margin-top: 2px;
}

.check small {
  margin-top: 2px;
}

select {
  font: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 8px 10px;
}

.hint {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
}

code {
  background: var(--bg);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 12px;
}

.presets {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.presets button {
  font-size: 12px;
  padding: 4px 10px;
}

.error {
  margin: 0;
  color: var(--bad);
  user-select: text;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.actions .reset {
  margin-right: auto;
  background: transparent;
  color: var(--muted);
}
</style>
