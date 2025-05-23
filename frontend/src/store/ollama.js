import { defineStore } from 'pinia'

export const useOllamaStore = defineStore('ollama', () => {
  const installed = ref(false)
  const started = ref(false)
  const canStart = ref(false)
  const version = ref('')
  const upgrade = ref(false)
  const lastVersion = ref({})

  return { installed, started, canStart, version }
})
