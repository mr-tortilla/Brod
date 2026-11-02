<script lang="ts" setup>
import {computed, onUnmounted, ref} from 'vue'
import {tunnel} from '../../wailsjs/go/models'
import {countryName, flag, formatBytes, formatDuration} from '../format'

const props = defineProps<{
  status: tunnel.Status
  pendingName: string // к чему подключаемся, пока ядро ещё не запущено (идёт обновление подписки)
  busy: boolean
  error: string
  needAdmin: boolean // показать кнопку перезапуска с правами администратора (Windows)
}>()

const mode = defineModel<string>('mode', {required: true})

const emit = defineEmits<{
  disconnect: []
  relaunchAdmin: []
}>()

const now = ref(Date.now())
const timer = setInterval(() => now.value = Date.now(), 1000)
onUnmounted(() => clearInterval(timer))

const connected = computed(() => props.status.state === 'connected')
const connecting = computed(() => props.busy || props.status.state === 'connecting')
// туннель поднят, но запрос через него не проходит
const lost = computed(() => connected.value && props.status.latencyMs < 0)

const stateLabel = computed(() => {
  if (connecting.value) return 'Подключение…'
  if (lost.value) return 'Нет связи'
  return connected.value ? 'Подключено' : 'Отключено'
})

const nodeLabel = computed(() => {
  if (props.busy && props.pendingName) return props.pendingName
  if (connected.value || connecting.value) return props.status.nodeName
  return ''
})

const latency = computed(() => {
  const ms = props.status.latencyMs
  if (!ms) return '…'
  return ms < 0 ? 'нет ответа' : `${ms} мс`
})

const location = computed(() => {
  const s = props.status
  if (!s.countryCode) return ''
  return [flag(s.countryCode), countryName(s.countryCode), s.city && `· ${s.city}`].filter(Boolean).join(' ')
})
</script>

<template>
  <section class="status" :class="{connected: connected && !lost, connecting, lost}">
    <div class="state">
      <span class="dot"/>
      <span>{{ stateLabel }}</span>
      <span v-if="connected" class="uptime">{{ formatDuration(now - status.since) }}</span>
    </div>

    <div v-if="nodeLabel" class="node" :title="connected ? status.server : ''">{{ nodeLabel }}</div>
    <div v-else class="hint">Нажмите на подключение в списке</div>

    <dl v-if="connected">
      <dt>IP</dt>
      <dd class="selectable">{{ status.ip || '…' }}</dd>
      <dt>Страна</dt>
      <dd>{{ location || '…' }}</dd>
      <dt>Задержка</dt>
      <dd :class="{bad: status.latencyMs < 0}">{{ latency }}</dd>
      <dt>↓ Входящий</dt>
      <dd>{{ formatBytes(status.downSpeed) }}/с <span class="total">{{ formatBytes(status.downTotal) }}</span></dd>
      <dt>↑ Исходящий</dt>
      <dd>{{ formatBytes(status.upSpeed) }}/с <span class="total">{{ formatBytes(status.upTotal) }}</span></dd>
    </dl>

    <p v-if="error" class="error">{{ error }}</p>
    <button v-if="needAdmin" class="primary" @click="emit('relaunchAdmin')">Перезапустить с правами администратора</button>

    <div class="modes">
      <button type="button" :class="{active: mode === 'proxy'}" :disabled="connecting"
              title="Системный прокси: браузеры и приложения, которые его поддерживают. Права не нужны."
              @click="mode = 'proxy'">Прокси
      </button>
      <button type="button" :class="{active: mode === 'tun'}" :disabled="connecting"
              title="TUN: весь трафик системы плюс системный прокси для браузеров. Нужны права администратора."
              @click="mode = 'tun'">TUN
      </button>
    </div>

    <button v-if="connected" class="connect danger-fill" @click="emit('disconnect')">Отключить</button>
  </section>
</template>

<style scoped>
.status {
  border-top: 1px solid var(--border);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.state {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--muted);
}

.connected .dot {
  background: var(--good);
  box-shadow: 0 0 6px var(--good);
}

.lost .dot {
  background: var(--bad);
  box-shadow: 0 0 6px var(--bad);
}

.lost .state {
  color: var(--bad);
}

.connecting .dot {
  background: var(--warn);
  animation: blink 1s infinite;
}

@keyframes blink {
  50% { opacity: .3; }
}

.uptime {
  margin-left: auto;
  color: var(--muted);
  font-weight: 400;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.hint {
  color: var(--muted);
  font-size: 12px;
}

.node {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

dl {
  margin: 0;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 10px;
  font-size: 12px;
}

dt {
  color: var(--muted);
}

dd {
  margin: 0;
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

dd.bad, .error {
  color: var(--bad);
}

.selectable, .error {
  user-select: text;
}

.total {
  color: var(--muted);
  margin-left: 4px;
}

.error {
  margin: 0;
  font-size: 12px;
  word-break: break-word;
}

.modes {
  display: flex;
  gap: 4px;
  background: var(--bg);
  padding: 3px;
  border-radius: var(--radius);
}

.modes button {
  flex: 1;
  padding: 4px;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
}

.modes button.active {
  background: var(--panel-2);
  color: var(--text);
}

.modes button:disabled {
  opacity: 1;
}

.connect {
  padding: 9px;
  font-weight: 600;
}

.danger-fill {
  background: var(--bad);
  border-color: var(--bad);
  color: #fff;
}
</style>
