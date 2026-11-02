<script lang="ts" setup>
// Геометрия буквы та же, что в tools/genicons (сетка 100×100).
defineProps<{ size?: number }>()
</script>

<template>
  <svg :width="size ?? 28" :height="size ?? 28" viewBox="0 0 100 100" aria-hidden="true">
    <rect x="6" y="6" width="88" height="88" rx="20" fill="var(--accent)"/>
    <path
        transform="translate(19.1 20) scale(0.6)"
        fill="#fff"
        d="M20 14H34V86H20Z
           M27 14H55A18 18 0 0 1 55 50H27Z M39 26V38H55A6 6 0 0 0 55 26Z
           M27 38H59A24 24 0 0 1 59 86H27Z M39 50V74H59A12 12 0 0 0 59 50Z"
    />
  </svg>
</template>
