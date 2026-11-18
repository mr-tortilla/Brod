<script lang="ts" setup>
import {ref} from 'vue'
import {CreateProfileFromJSON, CreateProfileFromURL} from '../../wailsjs/go/main/App'
import {main} from '../../wailsjs/go/models'
import {errText} from '../format'

const emit = defineEmits<{
  created: [result: main.CreateResult]
  close: []
}>()

const mode = ref<'url' | 'json'>('url')
const name = ref('')
const input = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  if (!input.value.trim()) {
    error.value = mode.value === 'url' ? 'Вставьте ссылку' : 'Вставьте конфиг'
    return
  }
  loading.value = true
  error.value = ''
  try {
    const create = mode.value === 'url' ? CreateProfileFromURL : CreateProfileFromJSON
    emit('created', await create(name.value, input.value))
  } catch (e) {
    error.value = errText(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="overlay" @mousedown.self="emit('close')">
    <form class="dialog" @submit.prevent="submit">
      <h2>Новый профиль</h2>

      <div class="tabs">
        <button type="button" :class="{active: mode === 'url'}" @click="mode = 'url'">URL / ссылки</button>
        <button type="button" :class="{active: mode === 'json'}" @click="mode = 'json'">Конфиг</button>
      </div>

      <label>
        Название
        <input v-model="name" placeholder="Необязательно"/>
      </label>

      <label v-if="mode === 'url'">
        Ссылка на подписку или ссылки VLESS, Hysteria2, TUIC (по одной на строку)
        <textarea v-model="input" rows="5" placeholder="https://sub.example.com/abc  или  vless://..., hysteria2://..., tuic://..." autofocus/>
      </label>
      <label v-else>
        Конфиг xray или sing-box (JSON), Clash или Mihomo (YAML)
        <textarea v-model="input" rows="12" placeholder='{"outbounds": [ ... ]}  или  proxies: ...'/>
      </label>

      <p v-if="error" class="error">{{ error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Отмена</button>
        <button type="submit" class="primary" :disabled="loading">
          {{ loading ? 'Загрузка…' : 'Добавить' }}
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
  width: min(560px, calc(100vw - 32px));
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

label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--muted);
  font-size: 13px;
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
</style>
