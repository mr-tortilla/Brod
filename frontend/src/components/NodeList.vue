<script lang="ts" setup>
import {computed} from 'vue'
import {node} from '../../wailsjs/go/models'
import type {PingState} from '../types'

const props = defineProps<{
  nodes: node.Node[]
  pings: Record<string, PingState>
  sortByPing: boolean
  pinned: string[]
  activeId: string // подключённое или подключающееся
  connectedId: string
}>()

const emit = defineEmits<{
  connect: [id: string]
  pin: [id: string, pinned: boolean]
  share: [id: string]
}>()

// Закреплённые всегда сверху в порядке закрепления, сортировка по пингу их не трогает.
const sorted = computed(() => {
  const byId = new Map(props.nodes.map(n => [n.id, n]))
  const pinned = props.pinned.map(id => byId.get(id)).filter((n): n is node.Node => !!n)
  const pinnedSet = new Set(props.pinned)
  const rest = props.nodes.filter(n => !pinnedSet.has(n.id))
  if (props.sortByPing) {
    const weight = (n: node.Node) => {
      const p = props.pings[n.id]
      if (!p || p === 'pending') return Number.MAX_SAFE_INTEGER - 1
      return p.ms < 0 ? Number.MAX_SAFE_INTEGER : p.ms
    }
    rest.sort((a, b) => weight(a) - weight(b))
  }
  return [...pinned, ...rest]
})

const pinnedSet = computed(() => new Set(props.pinned))

function transport(n: node.Node) {
  const parts = [n.network]
  if (n.security !== 'none') parts.push(n.security)
  if (n.flow) parts.push('vision')
  return parts.join(' · ')
}

function pingText(p?: PingState) {
  if (!p || p === 'pending') return '…'
  return p.ms < 0 ? p.error : `${p.ms} мс`
}

function pingClass(p?: PingState) {
  if (!p || p === 'pending') return 'pending'
  if (p.ms < 0) return 'bad'
  if (p.ms < 150) return 'good'
  if (p.ms < 400) return 'warn'
  return 'bad'
}
</script>

<template>
  <div class="list">
    <div
        v-for="n in sorted" :key="n.id"
        class="row" :class="{active: n.id === activeId, pinned: pinnedSet.has(n.id)}"
        @click="emit('connect', n.id)"
    >
      <button class="pin share" title="Поделиться" @click.stop="emit('share', n.id)">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"
             stroke-linecap="round" aria-hidden="true">
          <circle cx="18" cy="5" r="3"/>
          <circle cx="6" cy="12" r="3"/>
          <circle cx="18" cy="19" r="3"/>
          <path d="M8.6 13.5l6.8 4M15.4 6.5l-6.8 4"/>
        </svg>
      </button>
      <button
          class="pin" :class="{on: pinnedSet.has(n.id)}"
          :title="pinnedSet.has(n.id) ? 'Открепить' : 'Закрепить вверху'"
          @click.stop="emit('pin', n.id, !pinnedSet.has(n.id))"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
          <path d="M16 3l5 5-3 1-4 4 1 5-2 2-4-4-5 5-1-1 5-5-4-4 2-2 5 1 4-4z" fill="currentColor"/>
        </svg>
      </button>
      <div class="info">
        <div class="name"><span v-if="n.id === connectedId" class="live"/>{{ n.name }}</div>
        <div class="meta">{{ n.server }}:{{ n.port }}</div>
      </div>
      <span class="tag">{{ transport(n) }}</span>
      <span class="ping" :class="pingClass(pings[n.id])">{{ pingText(pings[n.id]) }}</span>
    </div>
  </div>
</template>

<style scoped>
.list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  cursor: pointer;
}

.row:hover {
  border-color: var(--border-strong);
}

.row.active {
  border-color: var(--accent);
  background: var(--panel-2);
}

.row.pinned + .row:not(.pinned) {
  margin-top: 8px;
}

.pin {
  padding: 4px;
  border: none;
  background: transparent;
  color: var(--muted);
  opacity: 0;
  display: flex;
}

.row:hover .pin {
  opacity: .6;
}

.pin:hover {
  opacity: 1 !important;
  color: var(--text);
}

.pin.on {
  opacity: 1;
  color: var(--accent);
}

.live {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 8px;
  vertical-align: middle;
  border-radius: 50%;
  background: var(--good);
  box-shadow: 0 0 6px var(--good);
}

.info {
  flex: 1;
  min-width: 0;
}

.name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.meta {
  color: var(--muted);
  font-size: 12px;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tag {
  color: var(--muted);
  font-size: 12px;
  background: var(--panel-2);
  padding: 3px 8px;
  border-radius: 6px;
  white-space: nowrap;
}

.ping {
  width: 140px;
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ping.pending { color: var(--muted); }
.ping.good { color: var(--good); }
.ping.warn { color: var(--warn); }
.ping.bad { color: var(--bad); }
</style>
