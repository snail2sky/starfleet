<template>
  <div class="shell">
    <aside class="sidebar" v-if="!isLogin">
      <div class="brand">
        <div class="brand__mark">S</div>
        <div class="brand__text">
          <div class="brand__name">Starfleet</div>
          <div class="brand__sub">Process Manager</div>
        </div>
      </div>

      <nav class="nav">
        <RouterLink
          v-for="item in nav"
          :key="item.path"
          :to="item.path"
          class="nav__item"
          :class="{ 'nav__item--active': isActive(item.path) }"
        >
          <span class="nav__icon" v-html="item.icon"></span>
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="sidebar__foot">
        <div class="health" :class="{ 'health--down': !serverUp }">
          <span class="health__dot"></span>
          {{ serverUp ? 'Server online' : 'Server unreachable' }}
        </div>
        <div class="sidebar__ver">starfleet v{{ version }}</div>
      </div>
    </aside>

    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { getHealth } from '@/api'

const route = useRoute()
const serverUp = ref(true)
const version = ref('1.0')
const isLogin = computed(() => route.name === 'login')

const nav = [
  {
    path: '/',
    label: 'Overview',
    icon: '<svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="1.5" y="1.5" width="5.5" height="5.5" rx="1.2"/><rect x="9" y="1.5" width="5.5" height="5.5" rx="1.2"/><rect x="1.5" y="9" width="5.5" height="5.5" rx="1.2"/><rect x="9" y="9" width="5.5" height="5.5" rx="1.2"/></svg>',
  },
  {
    path: '/processes',
    label: 'Processes',
    icon: '<svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="1.5" y="2.5" width="13" height="11" rx="1.5"/><path d="M4.5 6.5l2 1.8-2 1.8M8.5 10.5h3"/></svg>',
  },
  {
    path: '/schedules',
    label: 'Schedules',
    icon: '<svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="6.3"/><path d="M8 4.5V8l2.4 1.6"/></svg>',
  },
  {
    path: '/logs',
    label: 'Logs',
    icon: '<svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M2.5 3.5h11M2.5 6.5h7M2.5 9.5h11M2.5 12.5h7"/></svg>',
  },
  {
    path: '/settings',
    label: 'Settings',
    icon: '<svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="2.3"/><path d="M8 1.8v1.7M8 12.5v1.7M14.2 8h-1.7M3.5 8H1.8M12.4 3.6l-1.2 1.2M4.8 11.2l-1.2 1.2M12.4 12.4l-1.2-1.2M4.8 4.8L3.6 3.6"/></svg>',
  },
]

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

let timer: ReturnType<typeof setInterval>
onMounted(() => {
  const ping = async () => {
    if (isLogin.value) return
    try {
      await getHealth()
      serverUp.value = true
    } catch {
      serverUp.value = false
    }
  }
  ping()
  timer = setInterval(ping, 10000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.shell {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

/* ---------- sidebar ---------- */
.sidebar {
  width: 214px;
  flex-shrink: 0;
  background: var(--surface);
  border-right: 1px solid var(--line);
  display: flex;
  flex-direction: column;
  padding: 18px 12px 14px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 4px 8px 20px;
}
.brand__mark {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  background: linear-gradient(135deg, var(--accent), #2a78d6);
  color: #fff;
  font-weight: 700;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 14px rgba(57, 135, 229, 0.35);
}
.brand__name {
  font-size: 15px;
  font-weight: 650;
  color: var(--ink);
  letter-spacing: 0.01em;
  line-height: 1.2;
}
.brand__sub {
  font-size: 10.5px;
  color: var(--ink-3);
  letter-spacing: 0.07em;
  text-transform: uppercase;
  margin-top: 2px;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 3px;
  flex: 1;
}
.nav__item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 9px 11px;
  border-radius: var(--r-sm);
  color: var(--ink-3);
  font-size: 13.5px;
  font-weight: 500;
  text-decoration: none;
  position: relative;
  transition: background 0.15s, color 0.15s;
}
.nav__item:hover {
  background: var(--surface-2);
  color: var(--ink-2);
}
.nav__item--active {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}
.nav__item--active::before {
  content: "";
  position: absolute;
  left: -12px;
  top: 7px;
  bottom: 7px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: var(--accent);
}
.nav__icon {
  display: flex;
  opacity: 0.9;
}

.sidebar__foot {
  border-top: 1px solid var(--line);
  padding-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.health {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: var(--ink-3);
  padding: 0 4px;
}
.health__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--good);
  box-shadow: 0 0 6px rgba(12, 163, 12, 0.7);
}
.health--down .health__dot {
  background: var(--crit);
  box-shadow: 0 0 6px rgba(208, 59, 59, 0.7);
}
.sidebar__ver {
  font-size: 11px;
  color: var(--line-2);
  padding: 0 4px;
}

/* ---------- main ---------- */
.main {
  flex: 1;
  overflow-y: auto;
  padding: 26px 30px 40px;
  min-width: 0;
}
</style>
