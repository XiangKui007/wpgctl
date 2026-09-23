<script>
/** 现场向导 ⑥：市政水厂 / 模型服务；可一键连部 center → device → 模型。 */
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
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="field-grid" style="margin-bottom:0.75rem">
            <div class="field full">
              <label>市政水厂包目录（可选）</label>
              <div class="path-row">
                <el-input
                  v-model="siteForm.paths.waterwork"
                  placeholder="浏览选择市政水厂包目录（独立包，非 platform 子目录）" />
                <el-button type="primary" plain size="small" @click="openPicker('waterworkDir', 'dir')">浏览</el-button>
              </div>
            </div>
            <div class="field full">
              <label>模型服务包目录（可选）</label>
              <div class="path-row">
                <el-input
                  v-model="siteForm.paths.intelligentModel"
                  placeholder="浏览选择模型服务包目录（独立包）" />
                <el-button type="primary" plain size="small" @click="openPicker('intelligentModelDir', 'dir')">浏览</el-button>
              </div>
            </div>
          </div>
          <p v-if="!standaloneModules.length" class="muted">未填目录可跳过。</p>
          <div class="actions" style="margin:0 0 0.75rem" :class="{ busy }">
            <el-button
              type="primary"
              :class="{ 'btn-active': activeJobKey === 'standalone-all' }"
              @click="runAllStandaloneDeploy"
              :disabled="busy || !deployableStandaloneJobs.length"
            >
              {{ activeJobKey === 'standalone-all' ? '部署中…' : '一键部署' }}
            </el-button>
            <span class="muted" style="font-size:0.85rem">将依次部署：{{ deployableStandaloneJobs.map((j) => j.label).join('、') || '（请先填目录）' }}</span>
          </div>
          <div class="actions" style="margin:0 0 0.75rem">
            <el-button
              type="primary"
              plain
              size="small"
              @click="patchAllStandaloneEnv"
              :disabled="busy || !standaloneModules.length"
            >
              批量更新市政/模型 .env IP
            </el-button>
          </div>
          <div class="module-deploy-list">
            <template v-for="m in fieldModules.standalone" :key="m.name">
              <template v-if="m.name === 'waterwork'">
                <div v-for="sub in WATERWORK_SUBSERVICES" :key="sub.id" class="module-deploy-row">
                  <div>
                    <strong>{{ sub.label }}</strong>
                    <span class="muted"> — {{ siteForm.paths[m.pathKey] || '（未配置，可跳过）' }}</span>
                    <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel(m.name, 'standalone') }}</span>
                    <span v-if="fieldModuleStatus['std-'+m.name+'-'+sub.id]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus['std-'+m.name+'-'+sub.id] }}</span>
                  </div>
                  <div class="module-deploy-actions">
                    <el-button
                      type="primary"
                      plain
                      size="small"
                      @click="openStandaloneEnvEditor(m.pathKey, sub.dirName)"
                      :disabled="busy || !siteForm.paths[m.pathKey] || !isServiceEnabled(m.name)"
                    >编辑 .env</el-button>
                    <el-button
                      type="success"
                      plain
                      size="small"
                      :class="{ 'btn-active': activeJobKey === 'std-' + m.name + '-' + sub.id }"
                      @click="runStandaloneDeploy(m.pathKey, m.name, sub.id)"
                      :disabled="busy || !siteForm.paths[m.pathKey] || !isServiceEnabled(m.name)"
                    >
                      {{ activeJobKey === 'std-' + m.name + '-' + sub.id ? '部署中…' : '部署' }}
                    </el-button>
                  </div>
                </div>
              </template>
              <div v-else class="module-deploy-row">
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
                    :class="{ 'btn-active': activeJobKey === 'std-' + m.name }"
                    @click="runStandaloneDeploy(m.pathKey, m.name)"
                    :disabled="busy || !siteForm.paths[m.pathKey] || !isServiceEnabled(m.name)"
                  >
                    {{ activeJobKey === 'std-' + m.name ? '部署中…' : '部署' }}
                  </el-button>
                </div>
              </div>
            </template>
          </div>
          <JobLogBox :lines="jobLogs" :running="!!activeJobKey" />
          <div class="actions">
            <el-button plain @click="goToStep(4)">返回</el-button>
            <el-button type="primary" @click="completeFieldStep('standalone', 6)" :disabled="!fieldStepDone.business">
              {{ standaloneModules.length ? '独立包已处理，进入 ⑦ Nginx' : '跳过，进入 ⑦ Nginx' }}
            </el-button>
          </div>
        </div>
</template>
