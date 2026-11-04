<script setup lang="ts">
// 背景选择面板：内置渐变 + 用户导入图片（切换 / 上传 / 删除 / 恢复默认）
import { computed, ref, watch } from 'vue'
import { NButton, NIcon, NModal, NPopconfirm, NSpin, useMessage } from 'naive-ui'
import { AddOutline, TrashOutline, RefreshOutline, ImageOutline, CheckmarkCircle } from '@vicons/ionicons5'
import { api } from '../api'
import { store } from '../store'
import { BUILTIN_BACKGROUNDS, type BackgroundItem } from '../types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'changed'): void
}>()

const message = useMessage()
const items = ref<BackgroundItem[]>([])
const loading = ref(false)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const show = computed({
  get: () => props.show,
  set: v => emit('update:show', v),
})

async function load() {
  loading.value = true
  try {
    const res = await api.getBackgrounds()
    items.value = res.items || []
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

watch(() => props.show, v => { if (v) load() })

const isCurrent = (id: string) => store.background.id === id

async function pick(id: string, url = '') {
  try {
    await api.setBackground(id)
    store.background = { id, url }
    message.success('背景已切换')
    emit('changed')
  } catch (e: any) {
    message.error('切换失败：' + (e?.message || e))
  }
}

function pickBuiltin(id: string) {
  pick(id)
}

function pickUser(item: BackgroundItem) {
  pick(item.id, api.backgroundURL(item))
}

// 从本地添加：Wails 模式走系统文件框；浏览器模式走文件选择上传
async function addImage() {
  if (api.mode === 'wails') {
    uploading.value = true
    try {
      const item = await api.importBackground()
      if (item) {
        await load()
        store.background = { id: item.id, url: api.backgroundURL(item) }
        emit('changed')
        message.success('背景已导入并应用')
      }
    } catch (e: any) {
      message.error('导入失败：' + (e?.message || e))
    } finally {
      uploading.value = false
    }
  } else {
    fileInput.value?.click()
  }
}

async function onFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  uploading.value = true
  try {
    const item = await api.uploadBackground(file)
    await load()
    store.background = { id: item.id, url: api.backgroundURL(item) }
    emit('changed')
    message.success('背景已上传并应用')
  } catch (e: any) {
    message.error('上传失败：' + (e?.message || e))
  } finally {
    uploading.value = false
  }
}

async function remove(item: BackgroundItem) {
  try {
    await api.deleteBackground(item.id)
    await load()
    if (isCurrent(item.id)) {
      store.background = { id: 'builtin:sakura', url: '' }
      emit('changed')
    }
    message.success('背景已删除')
  } catch (e: any) {
    message.error('删除失败：' + (e?.message || e))
  }
}

function resetDefault() {
  pick('builtin:sakura')
}
</script>

<template>
  <NModal v-model:show="show" preset="card" title="选择背景" style="width: 620px" class="bg-modal">
    <NSpin :show="loading">
      <div class="section-title">内置渐变</div>
      <div class="grid">
        <div
          v-for="b in BUILTIN_BACKGROUNDS"
          :key="b.id"
          class="cell"
          :class="{ active: isCurrent(b.id) }"
          @click="pickBuiltin(b.id)"
        >
          <div class="thumb" :style="{ backgroundImage: b.css }" />
          <div class="cell-name">{{ b.name }}</div>
          <NIcon v-if="isCurrent(b.id)" class="check" :component="CheckmarkCircle" :size="18" />
        </div>
      </div>

      <div class="section-title images-title">
        我的图片
        <NButton size="tiny" type="primary" ghost :loading="uploading" @click="addImage">
          <template #icon><NIcon :component="AddOutline" /></template>
          从本地添加
        </NButton>
      </div>
      <div v-if="items.length === 0" class="empty">
        <NIcon :component="ImageOutline" :size="20" />
        还没有图片。可导入自己的动漫壁纸（jpg/png/webp/gif，≤15MB）
      </div>
      <div v-else class="grid">
        <div
          v-for="item in items"
          :key="item.id"
          class="cell"
          :class="{ active: isCurrent(item.id) }"
          @click="pickUser(item)"
        >
          <img class="thumb img" :src="api.backgroundURL(item)" :alt="item.name" loading="lazy" />
          <div class="cell-name">{{ item.name }}</div>
          <NIcon v-if="isCurrent(item.id)" class="check" :component="CheckmarkCircle" :size="18" />
          <NPopconfirm @positive-click="remove(item)">
            <template #trigger>
              <button class="del" title="删除背景" @click.stop>
                <NIcon :component="TrashOutline" :size="14" />
              </button>
            </template>
            删除这张背景？
          </NPopconfirm>
        </div>
      </div>
    </NSpin>

    <input ref="fileInput" type="file" accept="image/jpeg,image/png,image/webp,image/gif" class="hidden-input" @change="onFileChange" />

    <template #footer>
      <div class="footer">
        <span class="tip">背景图仅保存在本机 data/backgrounds/</span>
        <NButton size="small" @click="resetDefault">
          <template #icon><NIcon :component="RefreshOutline" /></template>
          恢复默认
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--fg-dim);
  margin: 4px 0 10px;
}
.images-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 18px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}
.cell {
  position: relative;
  border-radius: 10px;
  overflow: hidden;
  border: 2px solid transparent;
  cursor: pointer;
  background: #fff;
  transition: border-color 0.15s, transform 0.15s;
}
.cell:hover {
  transform: translateY(-1px);
  border-color: #ffd0e2;
}
.cell.active {
  border-color: var(--accent);
  box-shadow: 0 4px 14px rgba(255, 111, 165, 0.25);
}
.thumb {
  width: 100%;
  height: 76px;
  background-size: cover;
  background-position: center;
  display: block;
}
.thumb.img {
  object-fit: cover;
}
.cell-name {
  font-size: 12px;
  padding: 5px 8px;
  color: var(--fg);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.check {
  position: absolute;
  top: 6px;
  left: 6px;
  color: var(--accent);
  background: #fff;
  border-radius: 50%;
}
.del {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.92);
  color: #e05c86;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s;
}
.cell:hover .del {
  opacity: 1;
}
.empty {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--fg-dim);
  font-size: 12px;
  padding: 16px;
  background: rgba(255, 240, 247, 0.6);
  border: 1px dashed var(--border-strong);
  border-radius: 10px;
}
.hidden-input {
  display: none;
}
.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.tip {
  font-size: 12px;
  color: var(--fg-dim);
}
</style>