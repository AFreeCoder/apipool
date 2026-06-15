<template>
  <BaseDialog
    :show="show"
    :title="t('admin.ops.requestLog.title')"
    width="full"
    @close="handleClose"
  >
    <div class="space-y-5">
      <div class="flex flex-col gap-3 border-b border-gray-200 pb-4 dark:border-dark-700 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
            {{ user?.email || '-' }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.ops.requestLog.targetUser', { id: user?.id ?? '-' }) }}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <span :class="statusBadgeClass">
            <span class="h-1.5 w-1.5 rounded-full" :class="status?.enabled ? 'bg-red-500' : 'bg-gray-400'"></span>
            {{ status?.enabled ? t('admin.ops.requestLog.recording') : t('admin.ops.requestLog.notRecording') }}
          </span>
          <span v-if="status?.enabled" class="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
            <Icon name="clock" size="xs" />
            {{ remainingLabel }}
          </span>
        </div>
      </div>

      <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
        <div class="space-y-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ops.requestLog.bytesUsed') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatBytes(selectedBytesUsed, 1) }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ops.requestLog.itemCount') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ selectedItemCount }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ops.requestLog.droppedCount') }}</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ selectedDroppedCount }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.ops.requestLog.truncated') }}</div>
              <div class="mt-1 text-sm font-semibold" :class="selectedTruncated ? 'text-amber-600 dark:text-amber-300' : 'text-gray-900 dark:text-gray-100'">
                {{ selectedTruncated ? t('common.yes') : t('common.no') }}
              </div>
            </div>
          </div>

          <div v-if="memory" class="rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
            {{ t('admin.ops.requestLog.memoryPrecheck', {
              used: formatBytes(memory.used_memory, 1),
              max: memory.maxmemory > 0 ? formatBytes(memory.maxmemory, 1) : '-',
              percent: memory.percent
            }) }}
            <span v-if="memory.guarded" class="ml-2 font-medium text-red-600 dark:text-red-300">
              {{ t('admin.ops.requestLog.memoryGuarded') }}
            </span>
          </div>

          <div v-if="selectedTruncated || selectedDroppedCount > 0" class="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
            {{ t('admin.ops.requestLog.retentionHint') }}
          </div>
        </div>

        <div class="space-y-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div>
            <label class="input-label">{{ t('admin.ops.requestLog.session') }}</label>
            <Select
              v-model="selectedSessionId"
              :options="sessionOptions"
              :placeholder="t('admin.ops.requestLog.selectSession')"
              searchable
              @change="handleSessionChange"
            />
          </div>

          <div class="grid grid-cols-2 gap-3 text-xs">
            <div>
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.ops.requestLog.startedAt') }}</span>
              <div class="mt-1 text-gray-900 dark:text-gray-100">{{ selectedStartedAtLabel }}</div>
            </div>
            <div>
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.ops.requestLog.expiresAt') }}</span>
              <div class="mt-1 text-gray-900 dark:text-gray-100">{{ selectedExpiresAtLabel }}</div>
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loadingStatus"
              @click="refreshAll"
            >
              <Icon name="refresh" size="sm" :class="loadingStatus ? 'animate-spin' : ''" />
              {{ t('common.refresh') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="!selectedSessionId || downloading"
              @click="downloadSelectedSession"
            >
              <Icon name="download" size="sm" />
              {{ t('admin.ops.requestLog.download') }}
            </button>
            <button
              v-if="status?.enabled"
              type="button"
              class="btn btn-danger btn-sm"
              :disabled="disabling"
              @click="disableRecording"
            >
              {{ disabling ? t('admin.ops.requestLog.disabling') : t('admin.ops.requestLog.disable') }}
            </button>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/50 dark:bg-amber-900/20">
        <div class="flex items-start gap-3">
          <Icon name="exclamationTriangle" size="md" class="mt-0.5 flex-shrink-0 text-amber-600 dark:text-amber-300" />
          <div class="min-w-0 flex-1 space-y-3">
            <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
              {{ t('admin.ops.requestLog.complianceNotice') }}
            </p>
            <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px]">
              <textarea
                v-model="reason"
                rows="2"
                class="input resize-y"
                :placeholder="t('admin.ops.requestLog.reasonPlaceholder')"
              ></textarea>
              <div class="space-y-2">
                <label class="flex items-start gap-2 text-xs text-amber-800 dark:text-amber-200">
                  <input v-model="acknowledged" type="checkbox" class="mt-0.5 rounded border-amber-300 text-primary-600 focus:ring-primary-500" />
                  <span>{{ t('admin.ops.requestLog.confirmThirtyMinutes') }}</span>
                </label>
                <label class="flex items-start gap-2 text-xs text-amber-800 dark:text-amber-200">
                  <input v-model="force" type="checkbox" class="mt-0.5 rounded border-amber-300 text-primary-600 focus:ring-primary-500" />
                  <span>{{ t('admin.ops.requestLog.forceRestart') }}</span>
                </label>
              </div>
            </div>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="enabling || !canEnable"
              @click="enableRecording"
            >
              <Icon v-if="enabling" name="refresh" size="sm" class="animate-spin" />
              {{ enabling ? t('admin.ops.requestLog.enabling') : t('admin.ops.requestLog.enable') }}
            </button>
          </div>
        </div>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
        <DataTable :columns="itemColumns" :data="items" :loading="loadingItems">
          <template #cell-timestamp="{ value, row }">
            <button
              type="button"
              class="text-left text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              @click="openItemDetail(row)"
            >
              {{ formatDateTime(value) }}
            </button>
          </template>

          <template #cell-model="{ row }">
            <div class="max-w-[220px] truncate" :title="row.model || row.path">
              <span class="font-medium">{{ row.model || '-' }}</span>
              <div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ row.method }} {{ row.path }}</div>
            </div>
          </template>

          <template #cell-status_code="{ value }">
            <span :class="statusCodeClass(value)">
              {{ value }}
            </span>
          </template>

          <template #cell-duration_ms="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ value }}ms</span>
          </template>

          <template #cell-truncated="{ row }">
            <span v-if="row.req_truncated || row.resp_truncated" class="inline-flex rounded-md bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
              {{ t('admin.ops.requestLog.truncatedShort') }}
            </span>
            <span v-else class="text-xs text-gray-400">-</span>
          </template>

          <template #cell-actions="{ row }">
            <button
              type="button"
              class="btn btn-ghost btn-sm"
              @click="openItemDetail(row)"
            >
              <Icon name="eye" size="sm" />
              {{ t('admin.ops.requestLog.detail') }}
            </button>
          </template>

          <template #empty>
            <div class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.ops.requestLog.noItems') }}
            </div>
          </template>
        </DataTable>
        <Pagination
          v-if="itemsPagination.total > 0"
          :page="itemsPagination.page"
          :total="itemsPagination.total"
          :page-size="itemsPagination.page_size"
          @update:page="handleItemsPageChange"
          @update:pageSize="handleItemsPageSizeChange"
        />
      </div>

      <div v-if="detailItem" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-3 flex items-start justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ t('admin.ops.requestLog.detailTitle', { seq: detailItem.seq }) }}
            </h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ detailItem.method }} {{ detailItem.path }} · {{ detailItem.status_code }} · {{ detailItem.duration_ms }}ms
            </p>
          </div>
          <button type="button" class="btn btn-ghost btn-sm" @click="detailItem = null">
            <Icon name="x" size="sm" />
          </button>
        </div>

        <div class="grid gap-3 lg:grid-cols-2">
          <DetailBlock
            :title="t('admin.ops.requestLog.requestBody')"
            :content="detailItem.req_body"
            :truncated="detailItem.req_truncated"
            :meta="detailItem.req_body_kind"
            @copy="copyText"
          />
          <DetailBlock
            :title="t('admin.ops.requestLog.responseBody')"
            :content="detailItem.resp_body"
            :truncated="detailItem.resp_truncated"
            @copy="copyText"
          />
          <DetailBlock
            :title="t('admin.ops.requestLog.requestHeaders')"
            :content="formatJSON(detailItem.req_headers || {})"
            @copy="copyText"
          />
          <DetailBlock
            :title="t('admin.ops.requestLog.responseHeaders')"
            :content="formatJSON(detailItem.resp_headers || {})"
            @copy="copyText"
          />
          <DetailBlock
            class="lg:col-span-2"
            :title="t('admin.ops.requestLog.metadata')"
            :content="formatJSON(detailMetadata)"
            @copy="copyText"
          />
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ReqLogEntry,
  ReqLogRedisMemoryStats,
  ReqLogSession,
  ReqLogStatus
} from '@/api/admin/ops'
import type { AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import type { SelectOption } from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { formatBytes, formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
  user: AdminUser | null
}>()

const emit = defineEmits<{
  close: []
  statusChange: [userId: number, status: ReqLogStatus]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const status = ref<ReqLogStatus | null>(null)
const sessions = ref<ReqLogSession[]>([])
const selectedSessionId = ref<string | null>(null)
const items = ref<ReqLogEntry[]>([])
const detailItem = ref<ReqLogEntry | null>(null)
const memory = ref<ReqLogRedisMemoryStats | null>(null)
const loadingStatus = ref(false)
const loadingItems = ref(false)
const enabling = ref(false)
const disabling = ref(false)
const downloading = ref(false)
const reason = ref('')
const acknowledged = ref(false)
const force = ref(false)
const nowTick = ref(Date.now())
let tickTimer: ReturnType<typeof setInterval> | null = null

const itemsPagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 0
})

const itemColumns = computed<Column[]>(() => [
  { key: 'timestamp', label: t('admin.ops.requestLog.columns.time'), sortable: false },
  { key: 'model', label: t('admin.ops.requestLog.columns.model'), sortable: false },
  { key: 'status_code', label: t('admin.ops.requestLog.columns.status'), sortable: false },
  { key: 'duration_ms', label: t('admin.ops.requestLog.columns.duration'), sortable: false },
  { key: 'truncated', label: t('admin.ops.requestLog.columns.truncated'), sortable: false },
  { key: 'actions', label: t('admin.ops.requestLog.columns.actions'), sortable: false }
])

const activeSessionAsHistory = computed<ReqLogSession | null>(() => {
  const current = status.value?.session
  if (!current) return null
  const stats = status.value?.stats
  return {
    user_id: current.user_id,
    session_id: current.session_id,
    started_at: unixSecondsToISOString(current.started_at),
    expires_at: unixSecondsToISOString(current.expires_at),
    cutoff_at: stats?.expires_at || unixSecondsToISOString(current.expires_at),
    bytes_used: stats?.bytes_used ?? 0,
    item_count: stats?.item_count ?? 0,
    truncated: stats?.truncated ?? false,
    dropped_count: stats?.dropped_count ?? 0,
    status: stats?.status || 'enabled',
    reason: current.reason
  }
})

const mergedSessions = computed(() => {
  const map = new Map<string, ReqLogSession>()
  const active = activeSessionAsHistory.value
  if (active) map.set(active.session_id, active)
  for (const session of sessions.value) {
    if (!map.has(session.session_id)) map.set(session.session_id, session)
  }
  return [...map.values()]
})

const sessionOptions = computed<SelectOption[]>(() =>
  mergedSessions.value.map((session) => ({
    value: session.session_id,
    label: `${session.status} · ${formatDateTime(session.started_at)} · ${session.session_id}`
  }))
)

const selectedSession = computed(() =>
  mergedSessions.value.find((session) => session.session_id === selectedSessionId.value) || null
)

const selectedBytesUsed = computed(() => selectedSession.value?.bytes_used ?? status.value?.stats?.bytes_used ?? 0)
const selectedItemCount = computed(() => selectedSession.value?.item_count ?? status.value?.stats?.item_count ?? 0)
const selectedDroppedCount = computed(() => selectedSession.value?.dropped_count ?? status.value?.stats?.dropped_count ?? 0)
const selectedTruncated = computed(() => selectedSession.value?.truncated ?? status.value?.stats?.truncated ?? false)
const selectedStartedAtLabel = computed(() => selectedSession.value?.started_at ? formatDateTime(selectedSession.value.started_at) : '-')
const selectedExpiresAtLabel = computed(() => selectedSession.value?.expires_at ? formatDateTime(selectedSession.value.expires_at) : '-')

const remainingSeconds = computed(() => {
  const expiresAt = status.value?.session?.expires_at
  if (!status.value?.enabled || !expiresAt) return 0
  return Math.max(0, Math.floor((expiresAt * 1000 - nowTick.value) / 1000))
})

const remainingLabel = computed(() => formatDuration(remainingSeconds.value))
const canEnable = computed(() => !!props.user?.id && reason.value.trim().length > 0 && acknowledged.value)

const statusBadgeClass = computed(() => [
  'inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium',
  status.value?.enabled
    ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
    : 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
])

const detailMetadata = computed(() => {
  if (!detailItem.value) return {}
  const item = detailItem.value
  return {
    user_id: item.user_id,
    session_id: item.session_id,
    seq: item.seq,
    request_id: item.request_id,
    client_request_id: item.client_request_id,
    timestamp: item.timestamp,
    inbound_endpoint: item.inbound_endpoint,
    stream: item.stream,
    transport: item.transport,
    account_id: item.account_id,
    platform: item.platform,
    client_ip: item.client_ip,
    error_detail: item.error_detail
  }
})

watch(
  () => [props.show, props.user?.id] as const,
  ([show]) => {
    if (show) {
      startTickTimer()
      void refreshAll()
      return
    }
    stopTickTimer()
    resetState()
  },
  { immediate: true }
)

watch(selectedSessionId, (sessionId, prev) => {
  if (!props.show || !sessionId || sessionId === prev) return
  itemsPagination.value.page = 1
  detailItem.value = null
  void loadItems()
})

onUnmounted(() => {
  stopTickTimer()
})

async function refreshAll() {
  if (!props.user?.id) return
  loadingStatus.value = true
  try {
    const [nextStatus, nextSessions] = await Promise.all([
      adminAPI.ops.getRequestLoggingStatus(props.user.id),
      adminAPI.ops.listRequestLogSessions(props.user.id)
    ])
    status.value = nextStatus
    memory.value = nextStatus.memory ?? memory.value
    sessions.value = nextSessions || []
    emit('statusChange', props.user.id, nextStatus)

    if (nextStatus.session?.session_id) {
      selectedSessionId.value = nextStatus.session.session_id
    } else if (!selectedSessionId.value && sessions.value.length > 0) {
      selectedSessionId.value = sessions.value[0].session_id
    }

    if (selectedSessionId.value) {
      await loadItems()
    } else {
      items.value = []
      itemsPagination.value = { page: 1, page_size: itemsPagination.value.page_size, total: 0, pages: 0 }
    }
  } catch (error: any) {
    showError(error, t('admin.ops.requestLog.failedToLoad'))
  } finally {
    loadingStatus.value = false
  }
}

async function loadItems() {
  if (!selectedSessionId.value) return
  loadingItems.value = true
  try {
    const response = await adminAPI.ops.listRequestLogItems(selectedSessionId.value, {
      page: itemsPagination.value.page,
      pageSize: itemsPagination.value.page_size
    })
    items.value = response.items || []
    itemsPagination.value = {
      page: response.page,
      page_size: response.page_size,
      total: response.total,
      pages: response.pages
    }
  } catch (error: any) {
    showError(error, t('admin.ops.requestLog.failedToLoadItems'))
  } finally {
    loadingItems.value = false
  }
}

async function openItemDetail(row: ReqLogEntry) {
  if (!selectedSessionId.value) return
  try {
    detailItem.value = await adminAPI.ops.getRequestLogItem(selectedSessionId.value, row.seq)
  } catch (error: any) {
    showError(error, t('admin.ops.requestLog.failedToLoadDetail'))
  }
}

async function enableRecording() {
  if (!props.user?.id || !canEnable.value) return
  enabling.value = true
  try {
    const response = await adminAPI.ops.enableRequestLogging(props.user.id, {
      ttlSeconds: 30 * 60,
      reason: reason.value.trim(),
      force: force.value
    })
    memory.value = response.memory ?? null
    reason.value = ''
    acknowledged.value = false
    force.value = false
    appStore.showSuccess(t('admin.ops.requestLog.enabledSuccess'))
    await refreshAll()
  } catch (error: any) {
    showError(error, t('admin.ops.requestLog.failedToEnable'))
  } finally {
    enabling.value = false
  }
}

async function disableRecording() {
  if (!props.user?.id) return
  disabling.value = true
  try {
    await adminAPI.ops.disableRequestLogging(props.user.id)
    appStore.showSuccess(t('admin.ops.requestLog.disabledSuccess'))
    await refreshAll()
  } catch (error: any) {
    showError(error, t('admin.ops.requestLog.failedToDisable'))
  } finally {
    disabling.value = false
  }
}

async function downloadSelectedSession() {
  if (!selectedSessionId.value) return
  downloading.value = true
  try {
    await adminAPI.ops.downloadRequestLogSession(selectedSessionId.value)
  } catch (error: any) {
    showError(error, t('admin.ops.requestLog.failedToDownload'))
  } finally {
    downloading.value = false
  }
}

function handleSessionChange(value: string | number | boolean | null) {
  selectedSessionId.value = value == null ? null : String(value)
}

function handleItemsPageChange(page: number) {
  itemsPagination.value.page = page
  void loadItems()
}

function handleItemsPageSizeChange(pageSize: number) {
  itemsPagination.value.page_size = pageSize
  itemsPagination.value.page = 1
  void loadItems()
}

function handleClose() {
  emit('close')
}

function resetState() {
  status.value = null
  sessions.value = []
  selectedSessionId.value = null
  items.value = []
  detailItem.value = null
  memory.value = null
  reason.value = ''
  acknowledged.value = false
  force.value = false
  itemsPagination.value = { page: 1, page_size: 20, total: 0, pages: 0 }
}

function startTickTimer() {
  stopTickTimer()
  nowTick.value = Date.now()
  tickTimer = setInterval(() => {
    nowTick.value = Date.now()
  }, 1000)
}

function stopTickTimer() {
  if (!tickTimer) return
  clearInterval(tickTimer)
  tickTimer = null
}

function unixSecondsToISOString(value: number): string {
  if (!value) return ''
  return new Date(value * 1000).toISOString()
}

function formatDuration(seconds: number): string {
  if (seconds <= 0) return '00:00'
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(rest).padStart(2, '0')}`
}

function statusCodeClass(statusCode: number): string {
  const base = 'inline-flex rounded-md px-2 py-0.5 text-xs font-medium'
  if (statusCode >= 500) return `${base} bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300`
  if (statusCode >= 400) return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
  return `${base} bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300`
}

function formatJSON(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function copyText(value: string) {
  void copyToClipboard(value, t('admin.ops.requestLog.copied'))
}

function showError(error: any, fallback: string) {
  appStore.showError(error?.message || fallback)
}

const DetailBlock = defineComponent({
  name: 'UserRequestLogDetailBlock',
  props: {
    title: { type: String, required: true },
    content: { type: String, required: true },
    truncated: { type: Boolean, default: false },
    meta: { type: String, default: '' }
  },
  emits: ['copy'],
  setup(blockProps, { emit }) {
    return () => h('details', { class: 'rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800', open: true }, [
      h('summary', { class: 'flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium text-gray-900 dark:text-gray-100' }, [
        h('span', blockProps.title),
        h('button', {
          type: 'button',
          class: 'btn btn-ghost btn-sm',
          onClick: (event: MouseEvent) => {
            event.preventDefault()
            event.stopPropagation()
            emit('copy', blockProps.content)
          }
        }, t('admin.ops.requestLog.copy'))
      ]),
      blockProps.truncated
        ? h('div', { class: 'mt-2 rounded bg-amber-100 px-2 py-1 text-xs text-amber-700 dark:bg-amber-900/30 dark:text-amber-300' }, t('admin.ops.requestLog.bodyTruncated'))
        : null,
      blockProps.meta
        ? h('div', { class: 'mt-2 text-xs text-gray-500 dark:text-dark-400' }, blockProps.meta)
        : null,
      h('pre', { class: 'mt-2 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-md bg-white p-3 text-xs text-gray-800 dark:bg-dark-900 dark:text-dark-100' }, blockProps.content || '-')
    ])
  }
})
</script>
