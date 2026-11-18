<script lang="ts" setup>
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {
  Connect,
  DeleteProfile,
  Disconnect,
  GetStatus,
  ListProfiles,
  PingProfile,
  RefreshProfile,
  GetSettings,
  RelaunchAsAdmin,
  SetPinned,
  TakeStartupAction,
  Version,
} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime'
import {store, tunnel} from '../wailsjs/go/models'
import AddProfileDialog from './components/AddProfileDialog.vue'
import NodeList from './components/NodeList.vue'
import StatusPanel from './components/StatusPanel.vue'
import SettingsDialog from './components/SettingsDialog.vue'
import Logo from './components/Logo.vue'
import ShareDialog from './components/ShareDialog.vue'
import {setTheme, type ThemePref} from './theme'
import type {PingState} from './types'
import {errText, formatBytes} from './format'

type Pending = { profileId: string, nodeId: string, name: string }
type ContextMenu = { x: number, y: number, profileId: string, confirmDelete: boolean }

const profiles = ref<store.Profile[]>([])
const selectedId = ref('')
// Пинги кэшируются по профилям: повторный выбор профиля не запускает проверку заново.
const pings = ref<Record<string, Record<string, PingState>>>({})
const pinging = ref<Record<string, boolean>>({})
const refreshing = ref(false)
const nodeSort = ref<'ping' | 'name' | 'country'>('ping')
const showAdd = ref(false)
const showSettings = ref(false)
const notice = ref('')

const status = ref<tunnel.Status>(tunnel.Status.createFrom({state: 'disconnected'}))
const mode = ref(loadSetting('mode', 'proxy'))
const pending = ref<Pending | null>(null) // подключение в процессе (обновление подписки + запуск ядра)
const connectError = ref('')
const needAdmin = ref<{ profileId: string, nodeId: string } | null>(null)
const menu = ref<ContextMenu | null>(null)
const version = ref('')
const sharing = ref<{ profileId: string, nodeId?: string } | null>(null)

const selected = computed(() => profiles.value.find(p => p.id === selectedId.value))

const connectedId = computed(() =>
    status.value.state === 'connected' && status.value.profileId === selectedId.value ? status.value.nodeId : '')

// Подсвечиваемое в списке: то, к чему подключаемся, иначе то, к чему подключены.
const activeId = computed(() => {
  if (pending.value) return pending.value.profileId === selectedId.value ? pending.value.nodeId : ''
  return status.value.state !== 'disconnected' && status.value.profileId === selectedId.value ? status.value.nodeId : ''
})

const menuProfile = computed(() => profiles.value.find(p => p.id === menu.value?.profileId))

const offs: (() => void)[] = []

onMounted(async () => {
  pingTimer = window.setInterval(() => {
    if (document.visibilityState === 'visible' && selectedId.value) ping(selectedId.value, true)
  }, PING_INTERVAL)
  offs.push(
      EventsOn('ping:result', (r: { profileId: string, nodeId: string, ms: number, error?: string }) => {
        (pings.value[r.profileId] ??= {})[r.nodeId] = {ms: r.ms, error: r.error}
      }),
      EventsOn('ping:done', (profileId: string) => {
        pinging.value[profileId] = false
      }),
      EventsOn('tunnel:status', (s: tunnel.Status) => {
        status.value = s
      }),
  )
  window.addEventListener('click', closeMenu)
  window.addEventListener('blur', closeMenu)
  window.addEventListener('keydown', onKey)

  GetSettings().then(st => setTheme(st.theme as ThemePref)).catch(() => setTheme('auto'))
  version.value = await Version()
  status.value = await GetStatus()
  profiles.value = await ListProfiles() ?? []

  // после перезапуска с правами администратора — сразу подключиться, куда собирались
  const act = await TakeStartupAction()
  if (act) {
    mode.value = act.mode
    saveSetting('mode', act.mode)
    select(act.profileId)
    const n = profiles.value.find(p => p.id === act.profileId)?.nodes.find(x => x.id === act.nodeId)
    if (n) connectTo(act.profileId, act.nodeId, n.name)
    return
  }
  const first = profiles.value.find(p => p.id === status.value.profileId) ?? profiles.value[0]
  if (first) select(first.id)
})

onUnmounted(() => {
  clearInterval(pingTimer)
  offs.forEach(off => off())
  window.removeEventListener('click', closeMenu)
  window.removeEventListener('blur', closeMenu)
  window.removeEventListener('keydown', onKey)
})

function closeMenu() {
  menu.value = null
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') closeMenu()
}

function openMenu(e: MouseEvent, profileId: string) {
  // не даём меню вылезти за правый/нижний край окна
  menu.value = {
    x: Math.min(e.clientX, window.innerWidth - 170),
    y: Math.min(e.clientY, window.innerHeight - 130),
    profileId,
    confirmDelete: false,
  }
}

function loadSetting(key: string, fallback: string): string {
  try {
    return localStorage.getItem(key) ?? fallback
  } catch {
    return fallback
  }
}

function saveSetting(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // настройки не критичны
  }
}

function select(id: string) {
  if (id === selectedId.value) return
  selectedId.value = id
  notice.value = ''
  if (!pings.value[id]) ping(id)
}

// background — плановая перепроверка: прежние значения остаются на экране, пока не придут новые.
async function ping(id = selectedId.value, background = false) {
  const p = profiles.value.find(x => x.id === id)
  if (!p || (background && pinging.value[id])) return
  if (!background) pings.value[id] = Object.fromEntries(p.nodes.map(n => [n.id, 'pending' as const]))
  pinging.value[id] = true
  try {
    await PingProfile(id)
  } catch (e) {
    pinging.value[id] = false
    if (!background) notice.value = errText(e)
  }
}

// Пинг открытого профиля раз в минуту, пока окно видно (в трее не тратим запросы к серверам).
const PING_INTERVAL = 60_000
let pingTimer: number | undefined

// Срок и трафик подписки из ответа сервера подписки.
const subInfo = computed(() => {
  const res: Record<string, { text: string, title: string, warn: boolean }> = {}
  for (const p of profiles.value) {
    const s = p.sub
    if (!s) continue
    const parts: string[] = []
    const title: string[] = []
    let warn = false
    if (s.expire > 0) {
      const end = new Date(s.expire * 1000)
      const left = end.getTime() - Date.now()
      parts.push(left > 0 ? `до ${end.toLocaleDateString('ru-RU')}` : `истекла ${end.toLocaleDateString('ru-RU')}`)
      title.push(`Подписка до ${end.toLocaleString('ru-RU')}`)
      warn ||= left < 3 * 86_400_000
    }
    if (s.total > 0) {
      const used = s.upload + s.download
      parts.push(`${formatBytes(used)} из ${formatBytes(s.total)}`)
      title.push(`Трафик: использовано ${formatBytes(used)} из ${formatBytes(s.total)}`)
      warn ||= used >= s.total * 0.9
    }
    if (parts.length) res[p.id] = {text: parts.join(' · '), title: title.join('\n'), warn}
  }
  return res
})

// upsert кладёт свежую версию профиля; пингует, только если появились новые подключения.
function upsert(profile: store.Profile, forcePing = false) {
  const i = profiles.value.findIndex(p => p.id === profile.id)
  if (i >= 0) profiles.value[i] = profile
  else profiles.value.push(profile)

  const known = pings.value[profile.id]
  if (forcePing || !known || profile.nodes.some(n => !known[n.id])) ping(profile.id)
}

function onCreated(result: { profile: store.Profile, skipped: number }) {
  showAdd.value = false
  upsert(result.profile)
  select(result.profile.id)
  if (result.skipped > 0) {
    notice.value = `Пропущено записей: ${result.skipped} (другие протоколы или ошибки разбора)`
  }
}

async function refresh(id = selectedId.value) {
  if (!id) return
  refreshing.value = true
  notice.value = ''
  try {
    upsert((await RefreshProfile(id)).profile, true)
  } catch (e) {
    notice.value = errText(e)
  } finally {
    refreshing.value = false
  }
}

async function remove(id: string) {
  const p = profiles.value.find(x => x.id === id)
  if (!p) return
  try {
    await DeleteProfile(p.id)
  } catch (e) {
    notice.value = errText(e)
    return
  }
  profiles.value = profiles.value.filter(x => x.id !== p.id)
  delete pings.value[p.id]
  if (selectedId.value === p.id) {
    selectedId.value = ''
    if (profiles.value.length) select(profiles.value[0].id)
  }
}

async function togglePin(nodeId: string, pinned: boolean) {
  try {
    upsert(await SetPinned(selectedId.value, nodeId, pinned))
  } catch (e) {
    notice.value = errText(e)
  }
}

// Смена режима при активном подключении сразу переподключает в новом режиме.
function setMode(m: string) {
  mode.value = m
  saveSetting('mode', m)
  const s = status.value
  if (s.state === 'connected' && s.mode !== m) connectTo(s.profileId, s.nodeId, s.nodeName)
}

// Клик по подключению: подключиться, а если уже подключены к другому — переподключиться.
function connect(nodeId: string) {
  const s = status.value
  if (s.state === 'connected' && s.profileId === selectedId.value && s.nodeId === nodeId) return
  const n = selected.value?.nodes.find(x => x.id === nodeId)
  if (n) connectTo(selectedId.value, nodeId, n.name)
}

async function connectTo(profileId: string, nodeId: string, name: string) {
  if (pending.value) return
  pending.value = {profileId, nodeId, name}
  connectError.value = ''
  needAdmin.value = null
  notice.value = ''
  try {
    const res = await Connect(profileId, nodeId, mode.value)
    upsert(res.profile)
    if (res.refreshError) notice.value = `Подписку обновить не удалось, подключено по сохранённым данным: ${res.refreshError}`
  } catch (e) {
    const msg = errText(e)
    if (msg === 'need-admin') {
      // Windows: TUN без прав администратора не поднять — предлагаем перезапуск
      needAdmin.value = {profileId, nodeId}
      connectError.value = 'Для режима TUN нужны права администратора.'
    } else {
      connectError.value = msg
    }
  } finally {
    pending.value = null
  }
}

async function relaunchAsAdmin() {
  if (!needAdmin.value) return
  try {
    await RelaunchAsAdmin(needAdmin.value.profileId, needAdmin.value.nodeId)
  } catch (e) {
    connectError.value = errText(e)
  }
}

async function disconnect() {
  connectError.value = ''
  await Disconnect()
}

function onSettingsSaved(res: { reconnected: boolean }) {
  showSettings.value = false
  notice.value = res.reconnected ? 'Настройки сохранены, подключение перезапущено' : ''
}

const sourceLabel: Record<string, string> = {url: 'подписка', text: 'ссылки', json: 'конфиг'}
</script>

<template>
  <div class="layout">
    <aside>
      <div class="brand">
        <Logo :size="26"/>
        <span>Brod</span>
        <span class="version">{{ version }}</span>
      </div>
      <div class="aside-head">
        <span>Профили</span>
        <button class="primary" @click="showAdd = true">+ Добавить</button>
      </div>
      <nav>
        <button
            v-for="p in profiles" :key="p.id"
            class="profile" :class="{active: p.id === selectedId}"
            @click="select(p.id)"
          @contextmenu.prevent="openMenu($event, p.id)"
        >
          <span class="profile-name">{{ p.name }}</span>
          <span class="profile-meta">{{ sourceLabel[p.source] }} · {{ p.nodes.length }}</span>
          <span v-if="subInfo[p.id]" class="profile-meta" :class="{warn: subInfo[p.id].warn}" :title="subInfo[p.id].title">
            {{ subInfo[p.id].text }}
          </span>
        </button>
        <p v-if="!profiles.length" class="empty">Пока пусто</p>
      </nav>
      <button class="bypass-btn" title="Исключения и DNS" @click="showSettings = true">
        Настройки
      </button>
      <StatusPanel
          :status="status"
          :pending-name="pending?.name ?? ''"
          :busy="!!pending"
          :error="connectError"
          :need-admin="!!needAdmin"
          :mode="mode"
          @relaunch-admin="relaunchAsAdmin"
          @update:mode="setMode"
          @disconnect="disconnect"
      />
    </aside>

    <main v-if="selected">
      <header>
        <div>
          <h1>{{ selected.name }}</h1>
          <div class="sub">
            {{ selected.nodes.length }} подключений
            <template v-if="selected.url"> · {{ selected.url }}</template>
          </div>
        </div>
        <div class="toolbar">
          <label class="sort-control">Сортировать
            <select v-model="nodeSort" aria-label="Сортировка подключений">
              <option value="ping">по пингу</option>
              <option value="country">по стране</option>
              <option value="name">по имени</option>
            </select>
          </label>
          <button :disabled="pinging[selected.id]" @click="ping()">{{ pinging[selected.id] ? 'Проверка…' : 'Пинг' }}</button>
          <button v-if="selected.source === 'url'" :disabled="refreshing" @click="refresh()">
            {{ refreshing ? 'Обновление…' : 'Обновить' }}
          </button>
        </div>
      </header>
      <p v-if="notice" class="notice">{{ notice }}</p>
      <NodeList
          :nodes="selected.nodes"
          :pings="pings[selected.id] ?? {}"
          :sort-mode="nodeSort"
          :pinned="selected.pinned ?? []"
          :active-id="activeId"
          :connected-id="connectedId"
          @connect="connect"
          @pin="togglePin"
          @share="id => sharing = {profileId: selected!.id, nodeId: id}"
      />
    </main>

    <main v-else class="placeholder">
      <p>Добавьте профиль: ссылку на подписку, VLESS / Hysteria2 / TUIC ссылки или конфиг.</p>
      <button class="primary" @click="showAdd = true">+ Добавить профиль</button>
    </main>

    <AddProfileDialog v-if="showAdd" @created="onCreated" @close="showAdd = false"/>
    <SettingsDialog v-if="showSettings" @saved="onSettingsSaved" @close="showSettings = false"/>
    <ShareDialog v-if="sharing" :profile-id="sharing.profileId" :node-id="sharing.nodeId" @close="sharing = null"/>

    <div v-if="menu && menuProfile" class="menu" :style="{left: menu.x + 'px', top: menu.y + 'px'}" @click.stop>
      <button @click="sharing = {profileId: menuProfile.id}; closeMenu()">Поделиться</button>
      <button v-if="menuProfile.source === 'url'" @click="refresh(menuProfile.id); closeMenu()">Обновить подписку</button>
      <button v-if="!menu.confirmDelete" class="danger" @click="menu.confirmDelete = true">Удалить</button>
      <button v-else class="danger confirm" @click="remove(menuProfile.id); closeMenu()">Точно удалить?</button>
    </div>
  </div>
</template>

<style scoped>
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 14px 0;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: .3px;
}

.version {
  margin-left: auto;
  color: var(--muted);
  font-size: 11px;
  font-weight: 400;
}

.menu {
  position: fixed;
  z-index: 20;
  min-width: 160px;
  padding: 4px;
  display: flex;
  flex-direction: column;
  background: var(--panel-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
}

.menu button {
  text-align: left;
  border: none;
  background: transparent;
  padding: 7px 10px;
}

.menu button:hover {
  background: var(--panel);
}

.menu button.danger:hover,
.menu button.confirm {
  color: var(--bad);
}

.bypass-btn {
  margin: 0 14px 10px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
}

.layout {
  display: flex;
  height: 100%;
}

aside {
  width: 260px;
  flex-shrink: 0;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}

.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
  font-weight: 600;
}

nav {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.profile {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  text-align: left;
  background: transparent;
  border-color: transparent;
}

.profile.active {
  background: var(--panel-2);
  border-color: var(--border);
}

.profile-name {
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.profile-meta, .empty, .sub {
  color: var(--muted);
  font-size: 12px;
}

.profile-meta.warn {
  color: var(--bad);
}

.empty {
  padding: 8px;
}

main {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 20px 24px;
}

header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

header > div:first-child {
  min-width: 0;
}

h1 {
  margin: 0 0 4px;
  font-size: 20px;
}

.sub {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  user-select: text;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.sort-control {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
}

.sort-control select {
  color: var(--text);
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 5px 7px;
}

.check {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  cursor: pointer;
}

.check input {
  width: auto;
}

.notice {
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--radius);
  background: var(--panel-2);
  color: var(--warn);
  user-select: text;
}

.placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--muted);
}
</style>
