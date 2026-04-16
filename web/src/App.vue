<template>
  <div id="app">
    <header class="app-header">
      <span class="logo">0200<span class="logo-suffix">::/7</span></span>
      <nav class="nav-tabs">
        <RouterLink to="/" class="tab" exact-active-class="tab--active">Network</RouterLink>
        <RouterLink to="/about" class="tab" active-class="tab--active">About</RouterLink>
        <RouterLink to="/source" class="tab" active-class="tab--active">Source</RouterLink>
      </nav>
      <button
        class="theme-btn"
        @click="themeStore.toggle()"
        :title="themeStore.isDark ? 'Switch to light mode' : 'Switch to dark mode'"
      >{{ themeStore.isDark ? '☀' : '🌙' }}</button>
    </header>
    <main class="app-content">
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { useThemeStore } from './stores/theme'
const themeStore = useThemeStore()
</script>

<style scoped>
#app {
  height: 100vh;
  overflow: hidden;
}

.app-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 48px;
  background: var(--header-bg);
  border-bottom: 1px solid var(--header-border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  z-index: 100;
  gap: 16px;
}

.logo {
  font-family: 'Inconsolata', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 32px;
  font-weight: 100;
  color: var(--logo-color);
  letter-spacing: 0;
  margin-right: 20px;
  line-height: 1;
}

.logo-suffix {
  color: var(--logo-suffix);
}

.nav-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
}

.tab {
  padding: 4px 12px;
  font-size: 14px;
  font-family: 'Open Sans', 'Segoe UI', sans-serif;
  color: var(--tab-color);
  text-decoration: none;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}

.tab:hover {
  color: var(--tab-active);
}

.tab--active {
  color: var(--tab-active);
  font-weight: 700;
  border-bottom-color: #29BBFF;
}

.theme-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  padding: 4px 6px;
  border-radius: 4px;
  opacity: 0.75;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.theme-btn:hover {
  opacity: 1;
}

.app-content {
  position: fixed;
  top: 48px;
  left: 0;
  right: 0;
  bottom: 0;
  overflow: hidden;
}

@media (max-width: 768px) {
  .app-header {
    height: 44px;
    padding: 0 10px;
    gap: 8px;
  }
  .logo {
    font-size: 22px;
    margin-right: 10px;
  }
  .app-content {
    top: 44px;
  }
  .tab {
    padding: 4px 8px;
    font-size: 13px;
  }
}

@media (max-width: 480px) {
  .app-header {
    height: 40px;
    padding: 0 8px;
    gap: 4px;
  }
  .logo {
    font-size: 18px;
    margin-right: 6px;
  }
  .app-content {
    top: 40px;
  }
  .tab {
    padding: 3px 6px;
    font-size: 12px;
  }
}
</style>
