<script setup lang="ts">
// AI 音乐：生成表单（描述/歌词/纯音乐 + 风格/情绪/语言/标题）+ 任务列表 + My Music 回听
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  NButton, NEmpty, NIcon, NInput, NSelect, NSpin, NTag, NPopconfirm,
  NCollapse, NCollapseItem, useMessage,
} from 'naive-ui'
import {
  SparklesOutline, PlayOutline, RefreshOutline, TrashOutline, SettingsOutline,
  CheckmarkCircle, AlertCircle, TimeOutline, SyncOutline,
} from '@vicons/ionicons5'
import { api } from '../api'
import { store } from '../store'
import {
  GEN_LANGS, GEN_MOODS, GEN_STYLES,
  type GenConfig, type GenMode, type GenTask,
} from '../types'

const message = useMessage()

// ---- 配置 ----
const genConfig = ref<GenConfig>({
  provider: 'local', base_url: '', api_key: '', model: '', defaults: {},
})
const configSaving = ref(false)

const providerOptions = [
  { label: '本地合成器（离线可用，无需 Key）', value: 'local' },
  { label: 'MiniMax 海螺音乐（官方/中转）', value: 'minimax' },
  { label: 'Suno（官方/中转）', value: 'suno' },
]
const isRemote = computed(() => genConfig.value.provider !== 'local')

async function loadConfig() {
  try {
    genConfig.value = await api.getGenConfig()
  } catch { /* 首次使用用默认 */ }
}

async function saveConfig() {
  configSaving.value = true
  try {
    await api.setGenConfig({
      provider: genConfig.value.provider,
      base_url: genConfig.value.base_url,
      api_key: genConfig.value.api_key,
      model: genConfig.value.model,
    })
    message.success('生成配置已保存')
  } catch (e: any) {
    message.error('保存失败：' + (e?.message || e))
  } finally {
    configSaving.value = false
  }
}

function onProviderChange(p: string) {
  const d = genConfig.value.defaults?.[p]
  if (d) {
    genConfig.value.base_url = d.base_url
    genConfig.value.model = d.model
  }
}

// ---- 生成表单 ----
const mode = ref<GenMode>('text')
const modeOptions: { label: string; value: GenMode; hint: string }[] = [
  { label: '文字生成', value: 'text', hint: '描述一段话，生成带人声的歌曲' },
  { label: '歌词生成', value: 'lyrics', hint: '把自己的歌词谱成歌曲' },
  { label: '纯音乐', value: 'instrumental', hint: '生成无人声的背景音乐' },
]
const prompt = ref('')
const lyrics = ref('')
const style = ref<string>('Pop')
const mood = ref<string>('平静')
const lang = ref<string>('中文')
const title = ref('')
const submitting = ref(false)

async function submit() {
  submitting.value = true
  try {
    const task = await api.genSubmit({
      mode: mode.value,
      prompt: prompt.value.trim(),
      lyrics: lyrics.value,
      style: style.value,
      mood: mood.value,
      lang: mode.value === 'instrumental' ? '纯音乐' : lang.value,
      title: title.value.trim(),
    })
    message.success('任务已提交，正在生成…')
    tasks.value.unshift(task)
    pollSoon()
  } catch (e: any) {
    message.error(e?.message || String(e))
  } finally {
    submitting.value = false
  }
}

// ---- 任务与 My Music ----
const tasks = ref<GenTask[]>([])
const loadingTasks = ref(false)
const lastActive = ref(0)

async function loadTasks() {
  loadingTasks.value = true
  try {
    const list = await api.genTasks()
    tasks.value = list || []
    lastActive.value = tasks.value.filter(isRunning).length
  } catch (e: any) {
    message.error('加载生成历史失败：' + (e?.message || e))
  } finally {
    loadingTasks.value = false
  }
}

function isRunning(t: GenTask) {
  return t.status === 'pending' || t.status === 'queued' || t.status === 'generating'
}

function statusTag(t: GenTask): { type: 'default' | 'info' | 'success' | 'error' | 'warning'; text: string } {
  switch (t.status) {
    case 'pending': return { type: 'default', text: '排队中' }
    case 'queued': return { type: 'info', text: '已提交' }
    case 'generating': return { type: 'warning', text: '生成中' }
    case 'done': return { type: 'success', text: '已完成' }
    case 'failed': return { type: 'error', text: '失败' }
    default: return { type: 'default', text: t.status }
  }
}

function statusIcon(t: GenTask) {
  switch (t.status) {
    case 'done': return CheckmarkCircle
    case 'failed': return AlertCircle
    case 'generating': return SyncOutline
    default: return TimeOutline
  }
}

// 回听：以 My Music 已入库歌曲为目标播放
async function playTask(t: GenTask) {
  if (t.status !== 'done' || !t.song_id) {
    message.info('该任务尚未生成完成')
    return
  }
  try {
    await api.play({ song_id: t.song_id })
  } catch (e: any) {
    message.error('播放失败：' + (e?.message || e))
  }
}

// 重新生成：把参数回填到表单
function reuseTask(t: GenTask) {
  mode.value = (t.mode as GenMode) || 'text'
  prompt.value = t.prompt
  lyrics.value = t.lyrics
  style.value = t.style || 'Pop'
  mood.value = t.mood || '平静'
  lang.value = t.lang || '中文'
  title.value = t.title
  message.info('参数已回填，修改后可再次生成')
  document.querySelector('.main-content')?.scrollTo?.({ top: 0, behavior: 'smooth' })
}

async function removeTask(t: GenTask) {
  try {
    await api.genDeleteTask(t.id)
    tasks.value = tasks.value.filter(x => x.id !== t.id)
    message.success('已删除（含入库歌曲）')
  } catch (e: any) {
    message.error('删除失败：' + (e?.message || e))
  }
}

// ---- 轮询：进行中任务实时刷新（HTTP 模式兜底；Wails 模式事件即时更新） ----
let timer: number | undefined
let offGen: (() => void) | undefined
let pollFast = false

function pollSoon() {
  pollFast = true
  window.setTimeout(async () => {
    await loadTasks()
    pollFast = false
  }, 1500)
}

onMounted(async () => {
  await Promise.all([loadConfig(), loadTasks()])
  timer = window.setInterval(() => {
    if (lastActive.value > 0 || pollFast) loadTasks()
  }, 2000)
  offGen = api.onGenEvent(async (name, task) => {
    // Wails 事件：更新对应任务状态（全局提示由 AppEvents 负责）
    const idx = tasks.value.findIndex(t => t.id === task.id)
    if (idx >= 0) tasks.value[idx] = task
    else await loadTasks()
    if (name === 'gen:done' || name === 'gen:failed') lastActive.value = 0
  })
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
  offGen?.()
})
</script>

<template>
  <div class="page">
    <div class="head">
      <h2 class="page-title"><NIcon :component="SparklesOutline" :size="20" /> AI 音乐</h2>
      <span class="sub">描述或歌词 → 生成歌曲 / 纯音乐，作品自动保存到 My Music</span>
    </div>

    <!-- 生成配置（可折叠） -->
    <NCollapse :default-expanded-names="isRemote ? ['cfg'] : []" class="cfg">
      <NCollapseItem title="生成设置（模型 / 中转地址 / API Key）" name="cfg">
        <div class="cfg-grid">
          <div class="cfg-item">
            <label>生成后端</label>
            <NSelect
              v-model:value="genConfig.provider"
              :options="providerOptions"
              @update:value="onProviderChange"
            />
          </div>
          <template v-if="isRemote">
            <div class="cfg-item">
              <label>接口地址（支持中转，可带路径/参数）</label>
              <NInput v-model:value="genConfig.base_url" placeholder="https://api.example.com/v1/music_generation" />
            </div>
            <div class="cfg-item">
              <label>模型名（中转可自定义）</label>
              <NInput v-model:value="genConfig.model" placeholder="music-01" />
            </div>
            <div class="cfg-item">
              <label>API Key</label>
              <NInput
                v-model:value="genConfig.api_key"
                type="password"
                show-password-on="click"
                placeholder="sk-…（保存在本机 data/config.json）"
              />
            </div>
          </template>
          <div class="cfg-foot">
            <span class="cfg-hint">
              {{ isRemote
                ? '官方与中转均可：填平台文档中的提交接口地址与 Key；生成内容版权与计费归对应平台'
                : '本地合成器离线可用（无需 Key），生成简单 BGM 级音乐，用于快速试听方向' }}
            </span>
            <NButton size="small" :loading="configSaving" @click="saveConfig">
              <template #icon><NIcon :component="SettingsOutline" /></template>
              保存设置
            </NButton>
          </div>
        </div>
      </NCollapseItem>
    </NCollapse>

    <!-- 生成表单 -->
    <div class="form-card">
      <div class="mode-tabs">
        <button
          v-for="m in modeOptions"
          :key="m.value"
          class="mode-tab"
          :class="{ active: mode === m.value }"
          @click="mode = m.value"
        >
          {{ m.label }}
        </button>
        <span class="mode-hint">{{ modeOptions.find(m => m.value === mode)?.hint }}</span>
      </div>

      <div class="form-body">
        <div class="form-left">
          <NInput
            v-if="mode === 'lyrics'"
            v-model:value="lyrics"
            type="textarea"
            :rows="8"
            placeholder="粘贴你的歌词，每行一句…"
          />
          <NInput
            v-else
            v-model:value="prompt"
            type="textarea"
            :rows="8"
            :placeholder="mode === 'text'
              ? '描述你想要的歌曲，例如：一首欢快的夏日流行歌，关于海边和青春回忆，女声'
              : '描述你想要的氛围，例如：安静的钢琴曲，适合深夜阅读，舒缓放松'"
          />
          <div class="form-row">
            <div class="field">
              <label>风格</label>
              <NSelect v-model:value="style" :options="GEN_STYLES.map(s => ({ label: s, value: s }))" />
            </div>
            <div class="field">
              <label>情绪</label>
              <NSelect v-model:value="mood" :options="GEN_MOODS.map(s => ({ label: s, value: s }))" />
            </div>
            <div class="field" v-if="mode !== 'instrumental'">
              <label>语言</label>
              <NSelect v-model:value="lang" :options="GEN_LANGS.map(s => ({ label: s, value: s }))" />
            </div>
          </div>
          <div class="form-row">
            <div class="field grow">
              <label>歌曲标题（可选）</label>
              <NInput v-model:value="title" placeholder="留空自动命名" />
            </div>
            <NButton
              type="primary"
              size="large"
              class="gen-btn"
              :loading="submitting"
              @click="submit"
            >
              <template #icon><NIcon :component="SparklesOutline" /></template>
              生成音乐
            </NButton>
          </div>
        </div>
      </div>
    </div>

    <!-- My Music 历史 -->
    <div class="head tasks-head">
      <h3 class="tasks-title">My Music · 生成历史</h3>
      <NButton size="small" quaternary :loading="loadingTasks" @click="loadTasks">
        <template #icon><NIcon :component="RefreshOutline" /></template>
        刷新
      </NButton>
    </div>

    <NSpin :show="loadingTasks">
      <div v-if="tasks.length === 0" class="empty">
        <NEmpty description="还没有生成作品，填写上方表单开始创作" />
      </div>
      <div v-else class="task-list">
        <div v-for="t in tasks" :key="t.id" class="task" :class="{ running: isRunning(t) }">
          <div class="task-main" @click="playTask(t)">
            <NIcon
              class="task-icon"
              :component="t.status === 'done' ? PlayOutline : statusIcon(t)"
              :size="18"
              :class="{ spin: isRunning(t) }"
            />
            <div class="task-info">
              <div class="task-title">{{ t.title }}</div>
              <div class="task-meta">
                <span>{{ t.mode === 'instrumental' ? '纯音乐' : t.mode === 'lyrics' ? '歌词生成' : '文字生成' }}</span>
                <span v-if="t.style">· {{ t.style }}</span>
                <span v-if="t.mood">· {{ t.mood }}</span>
                <span v-if="t.provider">· {{ t.provider }}</span>
              </div>
            </div>
          </div>
          <div class="task-right">
            <NTag :type="statusTag(t).type" size="small" round>{{ statusTag(t).text }}</NTag>
            <NButton
              v-if="t.status === 'failed'"
              size="tiny"
              quaternary
              :title="t.fail_reason"
              @click="message.error(t.fail_reason || '生成失败')"
            >
              原因
            </NButton>
            <NButton size="tiny" quaternary title="回填参数重新生成" @click="reuseTask(t)">
              <template #icon><NIcon :component="RefreshOutline" /></template>
            </NButton>
            <NPopconfirm @positive-click="removeTask(t)">
              <template #trigger>
                <NButton size="tiny" quaternary type="error" title="删除（含已入库歌曲）">
                  <template #icon><NIcon :component="TrashOutline" /></template>
                </NButton>
              </template>
              删除该作品及其入库歌曲？
            </NPopconfirm>
          </div>
        </div>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  padding-bottom: 20px;
}
.head {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.page-title {
  margin: 0;
  font-size: 18px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.sub {
  color: var(--fg-dim);
  font-size: 12px;
}
.cfg {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 4px 14px;
}
.cfg-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.cfg-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.cfg-item label {
  font-size: 12px;
  color: var(--fg-dim);
}
.cfg-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 4px;
}
.cfg-hint {
  font-size: 12px;
  color: var(--fg-dim);
}
.form-card {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.mode-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mode-tab {
  padding: 6px 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--fg-dim);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}
.mode-tab:hover {
  color: var(--fg);
  border-color: var(--border-strong);
}
.mode-tab.active {
  background: var(--accent-weak);
  border-color: var(--accent);
  color: var(--accent);
  font-weight: 600;
}
.mode-hint {
  font-size: 12px;
  color: var(--fg-dim);
  margin-left: 6px;
}
.form-row {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  margin-top: 10px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 130px;
}
.field.grow {
  flex: 1;
}
.field label {
  font-size: 12px;
  color: var(--fg-dim);
}
.gen-btn {
  min-width: 130px;
}
.tasks-head {
  margin-top: 4px;
}
.tasks-title {
  margin: 0;
  font-size: 15px;
}
.empty {
  padding: 30px 0;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
}
.task-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.task {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 10px 14px;
}
.task.running {
  border-color: rgba(255, 111, 165, 0.45);
}
.task-main {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 0;
  cursor: pointer;
}
.task-icon {
  color: var(--accent);
  flex-shrink: 0;
}
.task-icon.spin {
  animation: spin 1.4s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.task-info {
  min-width: 0;
}
.task-title {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.task-meta {
  font-size: 12px;
  color: var(--fg-dim);
  display: flex;
  gap: 4px;
}
.task-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
</style>