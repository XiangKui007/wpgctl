<script>
/** 现场向导 ④：中间件部署与 Nacos 导入。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldMiddleware',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 4 / 8</strong> — 中间件多为 compose 部署（无 .env）。部署 Kafka 时会自动改
            <code>KAFKA_ADVERTISED_LISTENERS</code>。部署 Nacos 后<strong>必须导入配置</strong>（选 nacos*.zip 上传，不解压），否则平台/水厂无法从 Nacos 拉配置。
          </div>
          <div class="actions" style="margin-bottom:0.75rem" :class="{ busy }">
            <el-button type="primary"
              :class="{ 'btn-active': activeJobKey === 'middleware-all' }"
              @click="runAllMiddlewareDeploy"
              :disabled="busy || !fieldPaths.middlewareRoot || !deployableMiddlewareModules.length"
            >
              {{ activeJobKey === 'middleware-all' ? '部署中…' : '部署全部' }}
            </el-button>
            <span class="muted" style="font-size:0.85rem">将依次部署：{{ deployableMiddlewareModules.map((m) => m.label).join('、') || '（无可部署项）' }}</span>
          </div>
          <div class="module-deploy-list">
            <template v-for="m in fieldModules.middleware" :key="m.name">
              <div class="module-deploy-row">
                <div>
                  <strong>{{ m.label }}</strong>
                  <span class="muted"> — {{ moduleDir('middleware', m.name) }}</span>
                  <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel(m.name, 'middleware') }}</span>
                  <span v-if="fieldModuleStatus[m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus[m.name] }}</span>
                </div>
                <div class="module-deploy-actions">
                  <el-button
                    type="success"
                    plain
                    size="small"
                    :class="{ 'btn-active': activeJobKey === 'middleware-' + m.name }"
                    @click="runModuleDeploy('middleware', m.name, false)"
                    :disabled="busy || !fieldPaths.middlewareRoot || !isServiceEnabled(m.name)"
                  >
                    {{ activeJobKey === 'middleware-' + m.name ? '部署中…' : '部署' }}
                  </el-button>
                </div>
              </div>
              <div
                v-if="m.name === 'nacos' && isServiceEnabled('nacos')"
                class="nacos-import-panel"
                :class="{ warn: fieldModuleStatus.nacos && !nacosImportDone, ok: nacosImportDone }"
              >
                <div class="nacos-import-head">
                  <strong>导入 Nacos 配置</strong>
                  <span v-if="nacosImportDone" class="badge green">已导入</span>
                  <span v-else-if="fieldModuleStatus.nacos" class="badge yellow">Nacos 已部署，尚未导入</span>
                  <span v-else class="badge yellow">先部署 Nacos，再导入</span>
                </div>
                <p class="muted" style="margin:0.35rem 0 0.65rem">
                  部署 Nacos 之后必须把 <code>nacos*.zip</code> 上传导入（不解压）。跳过这一步，后面的平台、市政水厂都会报错。
                </p>
                <div class="field full" style="margin-bottom:0.5rem">
                  <label class="req">nacos*.zip（可多选）</label>
                  <div class="path-row">
                    <el-input
                      :value="nacosConfigZipSummary"
                      readonly
                      placeholder="未选择；点浏览勾选多个 nacos*.zip" />
                    <el-button type="primary" plain size="small" @click="openPicker('nacosConfigZips', 'nacos-zip')">浏览</el-button>
                    <el-button
                      type="warning"
                      plain
                      size="small"
                      @click="clearNacosConfigZips"
                      :disabled="!nacosConfigZipList.length"
                    >清空</el-button>
                  </div>
                  <ul v-if="nacosConfigZipList.length" class="nacos-zip-list">
                    <li v-for="(z, i) in nacosConfigZipList" :key="z">
                      <code>{{ zipBaseName(z) }}</code>
                      <el-button type="danger" plain size="small" @click="removeNacosConfigZip(i)">移除</el-button>
                    </li>
                  </ul>
                </div>
                <el-button type="primary"
                  :class="{ 'btn-active': activeJobKey === 'nacos-import' }"
                  @click="runNacosImport"
                  :disabled="busy"
                >
                  {{ activeJobKey === 'nacos-import' ? '导入中…' : (nacosImportDone ? '重新导入' : '导入配置') }}
                </el-button>
              </div>
            </template>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'mw'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <el-button plain @click="goToStep(2)">返回</el-button>
            <el-button type="primary"
              @click="completeFieldStep('middleware', 4)"
              :disabled="!fieldStepDone.database || (isServiceEnabled('nacos') && !nacosImportDone)"
              :title="isServiceEnabled('nacos') && !nacosImportDone ? '请先导入 Nacos 配置' : ''"
            >
              {{ isServiceEnabled('nacos') && !nacosImportDone ? '请先导入 Nacos 配置' : '中间件就绪，进入 ⑤ 业务服务' }}
            </el-button>
          </div>
        </div>
</template>
