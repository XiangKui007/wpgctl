<script>
/** 交付单列表：本地 ~/.wpgctl/state 台账。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'HistoryView',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>交付单</h2>
          <p>本地 ~/.wpgctl/state 中的版本真相来源。</p>
        </div>
        <div class="actions" style="margin-top:0">
          <el-button type="primary" plain @click="loadHistory">刷新</el-button>
          <el-button type="primary" plain @click="exportDeliveries">导出全部</el-button>
        </div>
      </div>
      <div class="surface">
        <el-table v-if="deployments.length" :data="deployments.slice().reverse()" stripe style="width:100%">
          <el-table-column label="时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.time) }}</template>
          </el-table-column>
          <el-table-column label="标题 / 动作" min-width="140">
            <template #default="{ row }">{{ row.title || row.action }}</template>
          </el-table-column>
          <el-table-column label="站点" min-width="120">
            <template #default="{ row }">{{ row.siteName || row.siteCode }}</template>
          </el-table-column>
          <el-table-column prop="packageVersion" label="版本" width="100" />
          <el-table-column label="操作者" width="110">
            <template #default="{ row }">{{ row.operator || '—' }}</template>
          </el-table-column>
          <el-table-column label="耗时" width="80">
            <template #default="{ row }">{{ row.durationSec != null ? row.durationSec + 's' : '—' }}</template>
          </el-table-column>
          <el-table-column label="结果" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.success ? 'success' : 'danger'">{{ row.success ? 'ok' : 'fail' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="验收" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.acceptanceOk ? 'success' : 'warning'">{{ row.acceptanceOk ? 'ok' : '—' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="" width="110">
            <template #default="{ row }">
              <el-button type="primary" plain size="small" @click="openReport(row.id)">查看报告</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="尚无交付记录。" />
      </div>
    </section>
</template>
