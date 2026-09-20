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
          <div class="hint-banner">
            <strong>步骤 5 / 8</strong> — 须已导入 Nacos 配置。各 platform 服务目录通常含 <code>*.tar.zip</code>，需先展开为 <code>.tar</code> 再 <code>docker load</code>。点「编辑 .env」会自动在子目录查找；没有则打开空白，保存即新建。
          </div>
          <div class="actions" style="margin-top:0;margin-bottom:0.75rem">
            <el-button type="warning" plain size="small" @click="expandPlatformArchives(true)" :disabled="busy || !fieldPaths.platformRoot">
              批量解压 platform 全部 tar.zip
            </el-button>
            <el-button type="primary" plain size="small" @click="patchAllBusinessEnv" :disabled="busy || !fieldPaths.platformRoot">
              批量更新 platform .env IP
            </el-button>
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
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'biz'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <el-button plain @click="goToStep(3)">返回</el-button>
            <el-button type="primary" @click="completeFieldStep('business', 5)" :disabled="!fieldStepDone.middleware">
              平台业务就绪，进入 ⑥ 市政/模型
            </el-button>
          </div>
        </div>
</template>
