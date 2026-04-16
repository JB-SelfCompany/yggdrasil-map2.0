import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export const useThemeStore = defineStore('theme', () => {
  const isDark = ref(localStorage.getItem('yggmap-theme') === 'dark')

  watch(isDark, (dark) => {
    localStorage.setItem('yggmap-theme', dark ? 'dark' : 'light')
    document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light')
  }, { immediate: true })

  function toggle() { isDark.value = !isDark.value }

  return { isDark, toggle }
})
