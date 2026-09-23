<script>
/** 现场向导 ⑤：平台业务部署。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldBusiness',
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
              <label class="req">platform 根目录</label>
              <div class="path-row">
                <el-input v-model="fieldPaths.platformRoot" placeholder="如 /workspace/platform（含 public/device）" />
                <el-button type="primary" plain size="small" @click="openPicker('platformRoot', 'dir')">浏览</el-button>
              </div>
            </div>
          </div>
          <div class="actions" style="margin-top:0;margin-bottom:0.75rem" :class="{ busy }">
            <el-button type="primary"
              :class="{ 'btn-active': activeJobKey === 'business-all' }"
              @click="runAllBusinessDeploy"
              :disabled="busy || !fieldPaths.platformRoot || !deployableBusinessModules.length"
            >
              {{ activeJobKey === 'business-all' ? '部署中…' : '一键部署' }}
            </el-button>
            <span class="muted" style="font-size:0.85rem">将依次部署：{{ deployableBusinessModules.map((m) => m.label).join('、') || '（无可部署项）' }}</span>
          </div>
          <div class="actions" style="margin-top:0;margin-bottom:0.75rem">
            <el-button type="warning" plain size="small" @click="expandPlatformArchives(true)" :disabled="busy || !fieldPaths.platformRoot">
              批量解压 platform 全部 tar.zip
            </el-button>
            <el-button type="primary" plain size="small" @click="patchAllBusinessEnv" :disabled="busy || !fieldPaths.platformRoot">
              批量更新 platform .env IP
            </el-button>
            <span class="muted" style="font-size:0.85rem">部署单个模块时会自动解压该模块的 tar.zip，一般不用手动批量解压。</span>
          </div>
          <div class="module-deploy-list">
            <div v-for="m in fieldModules.business" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ moduleDir('business', m.name) }}</span>
                <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel(m.name, 'platform') }}</span>
                <span v-if="fieldModuleStatus['biz-'+m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus['biz-'+m.name] }}</span>
              </div>
              <div class="module-deploy-actions">
                <el-button
                  type="primary"
                  plain
                  size="small"
                  @click="openEnvEditor('business', m.name)"
                  :disabled="busy || !fieldPaths.platformRoot || !isServiceEnabled(m.name)"
                >编辑 .env</el-button>
                <el-button
                  type="success"
                  plain
                  size="small"
                  :class="{ 'btn-active': activeJobKey === 'business-' + m.name }"
                  @click="runModuleDeploy('business', m.name, true)"
                  :disabled="busy || !fieldPaths.platformRoot || !isServiceEnabled(m.name)"
                >
                  {{ activeJobKey === 'business-' + m.name ? '部署中…' : '部署' }}
                </el-button>
              </div>
            </div>
          </div>
          <JobLogBox :lines="jobLogs" :running="!!activeJobKey" />
          <div class="actions">
            <el-button plain @click="goToStep(3)">返回</el-button>
            <el-button type="primary" @click="completeFieldStep('business', 5)" :disabled="!fieldStepDone.middleware">
              平台业务就绪，进入 ⑥ 市政/模型
            </el-button>
          </div>
        </div>
</template>
