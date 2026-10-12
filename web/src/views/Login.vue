<template>
  <div class="login">
    <form class="login__card" @submit.prevent="submit">
      <div class="login__brand">
        <div class="login__mark">S</div>
        <div>
          <div class="login__title">Starfleet</div>
          <div class="login__sub">Sign in to continue</div>
        </div>
      </div>

      <label class="login__field">
        <span>Username</span>
        <input v-model="username" name="username" autocomplete="username" required autofocus />
      </label>
      <label class="login__field">
        <span>Password</span>
        <input
          v-model="password"
          name="password"
          type="password"
          autocomplete="current-password"
          required
        />
      </label>

      <p v-if="error" class="login__error">{{ error }}</p>

      <button class="btn btn--primary login__submit" type="submit" :disabled="busy">
        {{ busy ? 'Signing in…' : 'Sign in' }}
      </button>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { login } from '@/api'

const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await login(username.value, password.value)
    // Full reload so every view refetches with the new token.
    window.location.assign('/')
  } catch (e: any) {
    const status = e?.response?.status
    error.value =
      status === 401
        ? 'Invalid username or password'
        : status === 400
          ? 'Authentication is disabled on this server'
          : 'Cannot reach the server'
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.login {
  min-height: 70vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
.login__card {
  width: 330px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  padding: 28px 26px 26px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  box-shadow: 0 10px 34px rgba(0, 0, 0, 0.45);
}
.login__brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}
.login__mark {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--accent), #2a78d6);
  color: #fff;
  font-weight: 700;
  font-size: 17px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.login__title {
  font-size: 16px;
  font-weight: 650;
  color: var(--ink);
  line-height: 1.2;
}
.login__sub {
  font-size: 12px;
  color: var(--ink-3);
  margin-top: 2px;
}
.login__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.login__field span {
  font-size: 12px;
  color: var(--ink-3);
}
.login__field input {
  background: var(--surface-3);
  border: 1px solid var(--line-2);
  border-radius: var(--r-sm);
  color: var(--ink);
  padding: 9px 11px;
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}
.login__field input:focus {
  border-color: var(--accent);
}
.login__error {
  margin: 0;
  font-size: 12.5px;
  color: var(--crit);
  background: var(--crit-soft);
  border-radius: var(--r-sm);
  padding: 8px 10px;
}
.login__submit {
  margin-top: 4px;
  width: 100%;
  height: 36px;
  font-size: 14px;
}
</style>
