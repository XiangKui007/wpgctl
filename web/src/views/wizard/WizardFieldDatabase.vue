<script>
/** 现场向导 ③：数据库 compose 部署，可选执行本机 .sql。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldDatabase',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 3 / 8</strong> — 数据库模块无 <code>.env</code>，仅 <code>docker-compose</code>：有 zip 则解压，已有 .tar 则直接 load → compose up。
          </div>
          <div class="actions" style="margin-bottom:0.75rem" :class="{ busy }">
            <el-button type="primary"
              :class="{ 'btn-active': activeJobKey === 'database-all' }"
              @click="runAllDatabaseDeploy"
              :disabled="busy || !fieldPaths.middlewareRoot || !deployableDatabaseModules.length"
            >
              {{ activeJobKey === 'database-all' ? '部署中…' : '部署全部' }}
            </el-button>
            <span class="muted" style="font-size:0.85rem">将依次部署：{{ deployableDatabaseModules.map((m) => m.label).join('、') || '（无可部署项）' }}</span>
          </div>
          <div class="module-deploy-list">
            <div v-for="m in fieldDatabaseModules" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ moduleDir('database', m.name) }}</span>
                <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel(m.name, 'database') }}</span>
                <span v-if="fieldModuleStatus[m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus[m.name] }}</span>
              </div>
              <el-button
                type="success"
                plain
                size="small"
                :class="{ 'btn-active': activeJobKey === 'database-' + m.name }"
                @click="runModuleDeploy('database', m.name, false)"
                :disabled="busy || !fieldPaths.middlewareRoot || !isServiceEnabled(m.name)"
              >
                {{ activeJobKey === 'database-' + m.name ? '部署中…' : '部署' }}
              </el-button>
            </div>
          </div>
          <div class="nacos-import-panel" style="margin-top:1rem">
            <div class="nacos-import-head">
              <strong>可选：执行 SQL 文件</strong>
              <span class="badge yellow">非必做</span>
            </div>
            <p class="muted" style="margin:0.35rem 0 0.65rem">
              数据库容器起来之后，可从本机 Linux 磁盘勾选 <code>.sql</code>，按顺序打到 site.yaml 里的 MySQL 或 PostgreSQL。不选也能进入下一步。失败不会回滚已执行语句。
            </p>
            <div class="field" style="margin-bottom:0.65rem">
              <label>目标库</label>
              <el-radio-group v-model="sqlApplyDriver" size="small">
                <el-radio-button value="pgsql">PostgreSQL</el-radio-button>
                <el-radio-button value="mysql" :disabled="siteForm.middleware.mysql.disabled">MySQL</el-radio-button>
              </el-radio-group>
            </div>
            <div class="field" style="margin-bottom:0.65rem">
              <label>库名（可空）</label>
              <el-input
                v-model="sqlApplyDatabase"
                :placeholder="sqlApplyDriver === 'pgsql' ? '空则连 postgres' : '空则不切库，脚本里自己 USE'"
                clearable
              />
            </div>
            <div class="field full" style="margin-bottom:0.5rem">
              <label>.sql 文件（可多选）</label>
              <div class="path-row">
                <el-input
                  :model-value="sqlApplySummary"
                  readonly
                  placeholder="未选择；点浏览勾选本机 .sql"
                />
                <el-button type="primary" plain size="small" @click="openPicker('sqlApplyFiles', 'sql')">浏览</el-button>
                <el-button
                  type="warning"
                  plain
                  size="small"
                  @click="clearSqlApplyFiles"
                  :disabled="!sqlApplyFiles.length"
                >清空</el-button>
              </div>
              <ul v-if="sqlApplyFiles.length" class="nacos-zip-list">
                <li v-for="(z, i) in sqlApplyFiles" :key="z">
                  <code>{{ zipBaseName(z) }}</code>
                  <el-button type="danger" plain size="small" @click="removeSqlApplyFile(i)">移除</el-button>
                </li>
              </ul>
            </div>
            <el-button
              type="primary"
              :class="{ 'btn-active': activeJobKey === 'db-apply' }"
              @click="runApplySQL"
              :disabled="busy || !sqlApplyFiles.length"
            >
              {{ activeJobKey === 'db-apply' ? '执行中…' : '执行已选 SQL' }}
            </el-button>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'db'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <el-button plain @click="goToStep(1)">返回</el-button>
            <el-button type="primary" @click="completeFieldStep('database', 3)" :disabled="!fieldStepDone.docker">
              数据库就绪，进入 ④ 中间件
            </el-button>
          </div>
        </div>
</template>
