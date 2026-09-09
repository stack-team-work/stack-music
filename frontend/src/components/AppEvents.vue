<script setup lang="ts">
// 全局事件接收器：订阅扫描完成/进度事件，弹提示并让各视图重载。
import { onMounted, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import { api } from '../api'
import { store } from '../store'

const message = useMessage()

let offDone: (() => void) | undefined
let offProgress: (() => void) | undefined

onMounted(() => {
  offProgress = api.onScanProgress((p) => {
    // 进度由 status.scanning 轮询体现，这里仅保持连接
  })

  offDone = api.onScanDone((d) => {
    store.scanning = false
    store.scanVersion++
    if (d.ok) {
      const parts = [`新增 ${d.added}`, `更新 ${d.updated}`]
      if (d.removed > 0) parts.push(`移除 ${d.removed}`)
      message.success(`扫描完成：${parts.join('，')}`)
    } else {
      message.error('扫描失败')
    }
  })
})

onUnmounted(() => {
  offDone?.()
  offProgress?.()
})
</script>

<template>
  <span style="display: none" />
</template>