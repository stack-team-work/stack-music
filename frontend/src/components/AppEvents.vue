<script setup lang="ts">
// 全局事件接收器：订阅扫描与 AI 生成任务事件，弹提示并触发视图重载。
import { onMounted, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import { api } from '../api'
import { store } from '../store'

const message = useMessage()

let offDone: (() => void) | undefined
let offProgress: (() => void) | undefined
let offGen: (() => void) | undefined

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

  // AI 生成任务：完成/失败全局提示；完成后曲库自动重载（新歌已在 My Music 中）
  offGen = api.onGenEvent((name, task) => {
    if (name === 'gen:done') {
      store.scanVersion++
      message.success(`「${task.title}」生成完成，已加入 My Music`)
    } else if (name === 'gen:failed') {
      message.error(`「${task.title}」生成失败：${task.fail_reason || '未知原因'}`)
    }
  })
})

onUnmounted(() => {
  offDone?.()
  offProgress?.()
  offGen?.()
})
</script>

<template>
  <span style="display: none" />
</template>