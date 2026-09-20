<script>
/** 验收报告：iframe 打开 /api/report。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'ReportView',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>验收报告</h2>
          <p>基于最近一次成功交付或指定交付单生成。</p>
        </div>
        <div class="actions" style="margin-top:0">
          <el-button type="primary" plain tag="a" :href="reportUrl" target="_blank" rel="noopener">新窗口打开</el-button>
          <el-button plain @click="view = 'history'">返回交付单</el-button>
        </div>
      </div>
      <div class="surface report-frame-wrap">
        <iframe class="report-frame" :src="reportUrl" title="验收报告"></iframe>
        <div v-if="smokeRows.length" style="margin-top:1rem">
          <p class="muted">本次会话健康矩阵</p>
          <table class="table">
            <thead><tr><th>服务</th><th>端口</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="e in smokeRows" :key="'r-' + e.name">
                <td>{{ e.name }}</td>
                <td>{{ e.port || '—' }}</td>
                <td><span class="badge" :class="e.ok ? 'green' : 'red'">{{ e.ok ? 'ok' : 'fail' }}</span></td>
                <td class="muted">{{ e.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
</template>
