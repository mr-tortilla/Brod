<script lang="ts" setup>
import {computed, onMounted, ref} from 'vue'
import {SaveFile, ShareNode, ShareProfile} from '../../wailsjs/go/main/App'
import {ClipboardSetText} from '../../wailsjs/runtime'
import {share} from '../../wailsjs/go/models'
import {errText} from '../format'

const props = defineProps<{
  profileId: string
  nodeId?: string // если задан — делимся одним подключением
}>()

const emit = defineEmits<{ close: [] }>()

type Tab = 'link' | 'qr' | 'json'

const data = ref<share.Data | null>(null)
const tab = ref<Tab>('link')
const error = ref('')
const done = ref('') // «Скопировано» / «Сохранено»

const isNode = computed(() => !!props.nodeId)

const linkLabel = computed(() => {
  const d = data.value
  if (!d) return ''
  if (d.linkKind === 'subscription') return 'Ссылка на подписку — у получателя профиль будет обновляться'
  if (isNode.value) return 'Ссылка vless:// — её понимают все VLESS-клиенты'
  return `Ссылки vless:// по одной на строку (${d.link.split('\n').length})`
})

onMounted(async () => {
  try {
    data.value = props.nodeId
        ? await ShareNode(props.profileId, props.nodeId)
        : await ShareProfile(props.profileId)
  } catch (e) {
    error.value = errText(e)
  }
})

function flash(text: string) {
  done.value = text
  setTimeout(() => done.value = '', 1500)
}

async function copy(text: string) {
  error.value = ''
  if (await ClipboardSetText(text)) flash('Скопировано')
  else error.value = 'Не удалось скопировать в буфер обмена'
}

// имя файла из названия: эмодзи-флаги и спецсимволы выкидываем
function fileName(ext: string) {
  const base = (data.value?.title ?? '').replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-+|-+$/g, '')
  return `brod-${base || 'export'}.${ext}`
}

async function save(content: string, ext: string) {
  error.value = ''
  try {
    if (await SaveFile(fileName(ext), content)) flash('Сохранено')
  } catch (e) {
    error.value = errText(e)
  }
}
</script>

<template>
  <div class="overlay" @mousedown.self="emit('close')">
    <div class="dialog">
      <h2>Поделиться {{ isNode ? 'подключением' : 'профилем' }}</h2>
      <div v-if="data" class="title">{{ data.title }}</div>

      <p class="warn">
        Внутри — ключи доступа к серверам. Делитесь только с теми, кому доверяете:
        провайдер может ограничивать число устройств на одну подписку.
      </p>

      <div class="tabs">
        <button :class="{active: tab === 'link'}" @click="tab = 'link'">Ссылка</button>
        <button :class="{active: tab === 'qr'}" @click="tab = 'qr'">QR-код</button>
        <button :class="{active: tab === 'json'}" @click="tab = 'json'">JSON</button>
      </div>

      <p v-if="!data && !error" class="muted">Загрузка…</p>

      <template v-if="data && tab === 'link'">
        <div class="muted">{{ linkLabel }}</div>
        <textarea readonly :rows="isNode || data.linkKind === 'subscription' ? 4 : 10" :value="data.link"/>
        <div class="actions">
          <button class="primary" @click="copy(data.link)">Копировать</button>
        </div>
      </template>

      <template v-if="data && tab === 'qr'">
        <template v-if="data.qr">
          <div class="qr"><img :src="data.qr" alt="QR-код"/></div>
          <div class="muted center">
            Отсканируйте в мобильном клиенте: v2rayNG, Hiddify, Streisand, Happ.
            <template v-if="data.linkKind === 'subscription'">Внутри — ссылка на подписку.</template>
            <template v-else>Внутри — ссылка vless://.</template>
          </div>
          <div class="actions">
            <button @click="save(data.qr, 'png')">Сохранить PNG</button>
          </div>
        </template>
        <p v-else class="muted">{{ data.qrError }}</p>
      </template>

      <template v-if="data && tab === 'json'">
        <div class="muted">
          Конфиг sing-box (outbounds) — импортируется в Brod и клиенты на sing-box.
          <template v-if="data.skipped">Пропущено подключений, которые нельзя выразить в sing-box: {{ data.skipped }}.</template>
        </div>
        <textarea readonly rows="12" :value="data.json"/>
        <div class="actions">
          <button @click="save(data.json, 'json')">Сохранить файл</button>
          <button class="primary" @click="copy(data.json)">Копировать</button>
        </div>
      </template>

      <p v-if="error" class="error">{{ error }}</p>

      <div class="footer">
        <span class="done">{{ done }}</span>
        <button @click="emit('close')">Закрыть</button>
      </div>
    </div>
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
  width: min(560px, calc(100vw - 32px));
  max-height: calc(100vh - 32px);
  overflow-y: auto;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

h2 {
  margin: 0;
  font-size: 18px;
}

.title {
  margin-top: -6px;
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.warn {
  margin: 0;
  padding: 8px 12px;
  border-radius: var(--radius);
  background: var(--panel-2);
  color: var(--warn);
  font-size: 12px;
  line-height: 1.5;
}

.tabs {
  display: flex;
  gap: 4px;
  background: var(--bg);
  padding: 4px;
  border-radius: var(--radius);
}

.tabs button {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--muted);
}

.tabs button.active {
  background: var(--panel-2);
  color: var(--text);
}

.muted {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.center {
  text-align: center;
}

textarea {
  word-break: break-all;
}

.qr {
  display: flex;
  justify-content: center;
}

/* QR на белом: тёмная тема не должна мешать камере */
.qr img {
  width: 280px;
  height: 280px;
  background: #fff;
  border-radius: var(--radius);
  padding: 8px;
  image-rendering: pixelated;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.error {
  margin: 0;
  color: var(--bad);
  user-select: text;
}

.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.done {
  color: var(--good);
  font-size: 13px;
}
</style>
