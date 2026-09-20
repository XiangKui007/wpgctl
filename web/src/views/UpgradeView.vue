<script>
/** 升级 / 回滚：patch 目录升级与指定版本回滚。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'UpgradeView',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>升级 / 回滚</h2>
          <p>日常 1~3 服务补丁升级；失败自动回滚镜像（SQL 不自动回滚）。</p>
        </div>
      </div>
      <div class="surface">
        <p v-if="latestVersion" class="hint-banner">当前成功版本：<strong>{{ latestVersion }}</strong></p>
        <p v-else class="hint-banner">尚无成功部署记录，请先完成首次 deploy。</p>

        <h3 style="margin:1.25rem 0 0.75rem;font-family:var(--font-display)">补丁升级</h3>
        <div class="field-grid">
          <div class="field full">
            <label class="req">Patch 包目录</label>
            <div class="path-row">
              <el-input v-model="upgradeForm.patchDir" placeholder="选择含 manifest.yaml 的 patch 目录" />
              <el-button type="primary" plain size="small" @click="openPicker('patchDir', 'dir')">浏览</el-button>
            </div>
          </div>
        </div>
        <div class="actions">
          <el-button type="primary" @click="runUpgrade" :disabled="busy || !upgradeForm.patchDir">
            {{ busy ? '升级中…' : '执行升级' }}
          </el-button>
        </div>

        <hr class="sep" />

        <h3 style="margin:0 0 0.75rem;font-family:var(--font-display)">回滚</h3>
        <div class="field-grid">
          <div class="field">
            <label>目标版本（空=上一成功版本）</label>
            <el-input v-model="upgradeForm.rollbackTo" placeholder="例如 4.0.2" />
          </div>
        </div>
        <div class="actions">
          <el-button type="danger" plain @click="runRollback" :disabled="busy">执行回滚</el-button>
        </div>

        <div class="log-box" v-if="jobLogs.length">
          <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
        </div>
      </div>
    </section>
</template>
