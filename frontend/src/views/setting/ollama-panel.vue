<template>
  <div
    v-loading="loading"
    :element-loading-text="loadingOptions.text"
    :element-loading-spinner="loadingOptions.svg"
    :element-loading-svg-view-box="loadingOptions.svgViewBox"
    :element-loading-background="loadingOptions.background">
    <el-alert title="自定义Ollama服务端信息，默认为本机" :closable="false" center style="border-radius: 0;margin-bottom: 10px;"/>
    <el-form ref="ollamaFormRef" :model="ollamaFormData" :rules="ollamaFormRule" label-width="100px" label-position="left" @submit.prevent>
      <el-form-item label="协议" prop="scheme">
        <el-select v-model="ollamaFormData.scheme" placeholder="请选择协议" style="width: 100%">
          <el-option v-for="(scheme, index) in schemes" :key="index" :label="scheme" :value="scheme"/>
        </el-select>
      </el-form-item>
      <el-form-item label="主机地址" prop="host">
        <el-input v-model.trim="ollamaFormData.host" placeholder="请输入主机地址"/>
      </el-form-item>
      <el-form-item label="端口" prop="port">
        <el-input v-model.trim="ollamaFormData.port" placeholder="请输入端口"/>
      </el-form-item>
      <el-form-item label-width="0">
        <div style="text-align: center;width: 100%;">
          <el-button type="primary" @click="handleSubmitOllamaConfig">保存</el-button>
          <el-button @click="$refs.ollamaFormRef.resetFields()">重置</el-button>
        </div>
      </el-form-item>
    </el-form>
  </div>

  <el-divider content-position="center">Ollama Environment Variables</el-divider>
  <div v-if="ollamaEnvsLoading" v-loading="true" element-loading-text="Loading variables..." style="min-height: 100px;"></div>
  <div v-else-if="ollamaEnvsError" style="text-align: center; color: red; padding: 10px;">Error loading variables: {{ ollamaEnvsError }}</div>
  <div v-else>
    <el-table :data="ollamaEnvs" style="width: 100%">
      <el-table-column prop="Name" label="Name" width="180" />
      <el-table-column prop="Value" label="Current Value" width="180" />
      <el-table-column prop="Description" label="Description" />
      <el-table-column label="New Value" width="200">
        <template #default="scope">
          <el-input v-model="scope.row.newValue" placeholder="Enter new value" />
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="120" fixed="right">
        <template #default="scope">
          <el-button type="primary" size="small" @click="handleSetEnvVar(scope.row)" :loading="scope.row.loading">Set</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue' // Added nextTick
import { ElMessage, ElMessageBox } from 'element-plus' // Ensure ElMessageBox is imported
import { runQuietly } from '~/utils/wrapper.js'
import { OllamaConfigs, SaveOllamaConfigs } from '@/go/app/Config.js'
import loadingOptions from '~/utils/loading.js'

// Refs for Ollama server config
const loading = ref(false)
const emptyData = {
  scheme: 'http',
  host: '127.0.0.1',
  port: '11434'
}
const schemes = ['http', 'https']
const ollamaFormRef = ref(null)
const ollamaFormData = ref({ ...emptyData })
const ollamaFormRule = ref({
  scheme: [{ required: true, message: '请输入协议', trigger: 'change' }],
  host: [{ required: true, message: '请选择主机地址', trigger: 'blur' }],
  port: [{ required: true, message: '请输入主机端口', trigger: 'blur' },
    { validator: (rule, value, callback) => {
      value = parseInt(value)
      if (isNaN(value) || value < 0) {
        callback(new Error('主机端口不合法，必须为正整数'))
      } else {
        callback()
      }
    }, trigger: 'blur' }]
})

function handleSubmitOllamaConfig() {
  ollamaFormRef.value?.validate().then(_ => {
    loading.value = true
    runQuietly(() => SaveOllamaConfigs({
      scheme: ollamaFormData.value.scheme,
      host: ollamaFormData.value.host,
      port: ollamaFormData.value.port
    }), _ => ElMessage.success('保存Ollama配置成功'), _ => ElMessage.error('保存Ollama配置失败'), _ => { loading.value = false })
  })
}

onMounted(() => {
  loading.value = true
  runQuietly(OllamaConfigs, data => {
    ollamaFormData.value = { ...emptyData, ...data }
  }, _ => ElMessage.error('获取Ollama配置失败'), _ => {
    nextTick(_ => ollamaFormRef.value?.clearValidate())
    loading.value = false
  })
  // Fetch Ollama environment variables on mount
  fetchOllamaEnvs()
})

// --- Ollama Environment Variables Logic ---
const ollamaEnvs = ref([])
const ollamaEnvsLoading = ref(false)
const ollamaEnvsError = ref(null)

async function fetchOllamaEnvs() {
  ollamaEnvsLoading.value = true
  ollamaEnvsError.value = null
  try {
    // Ensure correct Go function path. Based on prompt: window.go.app.App.ollama.Envs()
    const data = await window.go.app.App.ollama.Envs()
    ollamaEnvs.value = data.map(env => ({ ...env, newValue: '', loading: false }))
  } catch (error) {
    console.error('Error fetching Ollama envs:', error)
    const errorMessage = error?.message || error || 'Failed to fetch variables'
    ollamaEnvsError.value = errorMessage
    ElMessage.error(String(errorMessage)) // Ensure string conversion
  } finally {
    ollamaEnvsLoading.value = false
  }
}

async function handleSetEnvVar(envVar) {
  if (!envVar.newValue || String(envVar.newValue).trim() === '') {
    ElMessage.warning('New value cannot be empty.')
    return
  }

  try {
    await ElMessageBox.confirm(
      `Setting '${envVar.Name}' requires administrator privileges. The system may ask for your password. Continue?`,
      'Administrator Privileges Required',
      {
        confirmButtonText: 'Continue',
        cancelButtonText: 'Cancel',
        type: 'warning',
      }
    )
    // User clicked "Continue"
    envVar.loading = true
    try {
      await window.go.app.App.systemApp.SetOllamaEnvVar(envVar.Name, envVar.newValue)
      ElMessage.success(`Environment variable '${envVar.Name}' set successfully. You may need to restart Ollama or the system for changes to take full effect.`)
      fetchOllamaEnvs() // Refresh the list
    } catch (error) {
      console.error('Error setting env var:', error)
      const errorMessage = error?.message || error || 'Unknown error'
      ElMessage.error(`Failed to set '${envVar.Name}': ${String(errorMessage)}`)
    } finally {
      envVar.loading = false
    }
  } catch (action) {
    // Catches ElMessageBox.confirm's promise rejection (user clicked "Cancel" or closed dialog)
    if (action === 'cancel' || action === 'close') {
      ElMessage.info('Operation cancelled by user.')
    } else {
      // Handle other unexpected errors from ElMessageBox itself, if any
      console.error('Error with confirmation dialog:', action)
      ElMessage.error('An unexpected error occurred with the confirmation dialog.')
    }
    // Ensure loading is false if it was set true before dialog or for any other reason.
    // This is important if the loading state could have been true before the dialog.
    // In this specific flow, envVar.loading is set *after* confirmation,
    // so it should already be false if we reach here. However, being explicit doesn't hurt.
    envVar.loading = false 
  }
}

</script>

<style lang="scss" scoped>
/* Add any specific styles if needed */
.el-divider {
  margin-top: 20px;
  margin-bottom: 20px;
}
</style>
