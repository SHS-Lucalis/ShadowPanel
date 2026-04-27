<template>
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
    <GCard :title="trans('dedicated_servers.title_view')">
      <GTable>
        <tbody>
          <tr>
            <td><strong>{{ trans('dedicated_servers.gdaemon_uptime') }}:</strong></td>
            <td>{{ daemonInfo?.base_info?.uptime || '—' }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.gdaemon_version') }}:</strong></td>
            <td>{{ versionLine }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.gdaemon_online_servers_count') }}:</strong></td>
            <td>{{ daemonInfo?.base_info?.online_servers_count || '0' }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.gdaemon_working_tasks_count') }}:</strong></td>
            <td>{{ daemonInfo?.base_info?.working_tasks_count || '0' }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.gdaemon_waiting_tasks_count') }}:</strong></td>
            <td>{{ daemonInfo?.base_info?.waiting_tasks_count || '0' }}</td>
          </tr>
        </tbody>
      </GTable>
    </GCard>

    <GCard :title="trans('main.details')">
      <GTable>
        <tbody>
          <tr>
            <td><strong>{{ trans('dedicated_servers.location') }}:</strong></td>
            <td>{{ node?.location || '—' }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.provider') }}:</strong></td>
            <td>{{ node?.provider || '—' }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.os') }}:</strong></td>
            <td>
              <GIcon :name="osIcon" class="mr-1" />
              {{ node?.os || '—' }}
            </td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.ip') }}:</strong></td>
            <td>{{ ipString || '—' }}</td>
          </tr>
          <tr>
            <td><strong>{{ trans('dedicated_servers.work_path') }}:</strong></td>
            <td class="break-all">{{ node?.work_path || '—' }}</td>
          </tr>
        </tbody>
      </GTable>
    </GCard>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { GCard, GTable, GIcon } from '@gameap/ui'
import { trans } from '@/i18n/i18n'

const props = defineProps({
    node: { type: Object, default: null },
    daemonInfo: { type: Object, default: null },
})

const versionLine = computed(() => {
    const v = props.daemonInfo?.version
    if (!v?.version) return '—'
    return v.compile_date ? `${v.version} (${v.compile_date})` : v.version
})

const osIcon = computed(() => {
    const os = String(props.node?.os || '').toLowerCase()
    if (os.startsWith('w')) return 'windows'
    if (os.startsWith('m')) return 'apple'
    return 'linux'
})

const ipString = computed(() => {
    const ip = props.node?.ip
    if (!Array.isArray(ip) || ip.length === 0) return ''
    return ip.join(', ')
})
</script>
