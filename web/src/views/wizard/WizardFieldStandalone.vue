<script>
/** 现场向导 ⑥：市政水厂 / 模型服务。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldStandalone',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <div class="hint-banner">
            <strong>步骤 6 / 8</strong> — 市政水厂、模型服务为<strong>独立交付包</strong>。市政水厂夹层下 <code>waterwork-center</code> 与 <code>waterwork-device</code> 都会部署；PgSQL 版仍把库连接写进 <code>MYSQL_*</code>。可先编辑 <code>.env</code>，再部署（先 <code>docker load java8.tar</code>，再 <code>compose up -d --build</code>）。
          </div>
          <p v-if="!standaloneModules.length" class="muted">
            未配置独立包目录。可在步骤 ① 填写 <code>waterwork-4.1.1</code> / <code>wpg-intelligent-model-4.1.2</code> 路径，或跳过本步。
          </p>
          <div class="module-deploy-list">
            <div v-for="m in fieldModules.standalone" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ siteForm.paths[m.pathKey] || '（未配置，可跳过）' }}</span>
                <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel(m.name, 'standalone') }}</span>
                <span v-if="fieldModuleStatus['std-'+m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus['std-'+m.name] }}</span>
              </div>
              <div class="module-deploy-actions">
                <el-button
                  type="primary"
                  plain
                  size="small"
                  @click="openStandaloneEnvEditor(m.pathKey)"
                  :disabled="busy || !siteForm.paths[m.pathKey] || !isServiceEnabled(m.name)"
                >编辑 .env</el-button>
                <el-button
                  type="success"
                  plain
                  size="small"
                  :class="{ 'btn-active': activeJobKey === 'standalone-' + m.name }"
                  @click="runStandaloneDeploy(m.pathKey, m.name)"
                  :disabled="busy || !siteForm.paths[m.pathKey] || !isServiceEnabled(m.name)"
                >
                  {{ activeJobKey === 'standalone-' + m.name ? '部署中…' : '部署' }}
                </el-button>
              </div>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'std'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <el-button plain @click="goToStep(4)">返回</el-button>
            <el-button type="primary" @click="completeFieldStep('standalone', 6)" :disabled="!fieldStepDone.business">
              {{ standaloneModules.length ? '独立包已处理，进入 ⑦ Nginx' : '跳过，进入 ⑦ Nginx' }}
            </el-button>
          </div>
        </div>
</template>
