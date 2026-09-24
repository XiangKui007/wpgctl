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
            <div v-for="m in fieldModules.business" :key="m.name" v-show="m.name !== 'monitor'" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ moduleDir('business', m.name) }}</span>
                <span v-if="isMultiNode" class="node-target">@ {{ moduleTargetLabel(m.name, 'platform') }}</span>
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
                  type="primary"
                  plain
                  size="small"
                  @click="openComposeEditor('business', m.name)"
                  :disabled="busy || !fieldPaths.platformRoot || !isServiceEnabled(m.name)"
                >编辑 compose</el-button>
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
          <el-collapse class="monitor-fold">
            <el-collapse-item title="监控" name="monitor">
            <p class="muted" style="margin:0 0 0.35rem">目录：{{ monitorLayout.dir || moduleDir('business', 'monitor') }}</p>
            <p v-if="monitorLayout.error" class="muted">{{ monitorLayout.error }}</p>
            <p v-else-if="fieldPaths.platformRoot && !monitorLayout.exists" class="muted">该目录还不存在。</p>
            <div class="actions" style="margin:0.5rem 0 0.75rem">
              <el-button type="primary" plain size="small" :disabled="busy || !monitorLayout.exists" @click="expandMonitorServices">解压 otherServices</el-button>
              <el-button type="primary" plain size="small" :disabled="busy || !monitorLayout.exists" @click="patchMonitorFiles()">一键修改地址</el-button>
              <el-button plain size="small" :disabled="!fieldPaths.platformRoot" @click="refreshMonitorLayout">重新解析目录</el-button>
            </div>
            <div class="module-deploy-list">
              <div v-for="f in monitorLayout.files" :key="f.id" class="module-deploy-row">
                <div>
                  <strong>{{ f.label }}</strong>
                  <span class="muted"> — {{ f.path || '未找到' }}</span>
                </div>
                <div class="module-deploy-actions">
                  <el-button
                    v-if="f.kind !== 'zip' && ['kafka','mysql','pgsql','redis','prometheus-yml','env'].includes(f.id)"
                    type="primary"
                    plain
                    size="small"
                    :disabled="busy || !f.exists"
                    @click="patchMonitorFiles([f.id])"
                  >改地址</el-button>
                  <el-button
                    v-if="f.kind === 'zip'"
                    type="warning"
                    plain
                    size="small"
                    :disabled="busy"
                    @click="expandMonitorServices"
                  >解压</el-button>
                  <el-button
                    v-else
                    type="primary"
                    plain
                    size="small"
                    :disabled="!f.exists"
                    @click="openMonitorFile(f.path)"
                  >修改</el-button>
                </div>
              </div>
            </div>
            <p class="muted" style="margin:0.75rem 0">监控平台和 Prometheus 部署到 {{ moduleTargetLabel('monitor', 'platform') }}。node、cadvisor 部署到勾选的机器。Kafka / MySQL / PostgreSQL / Redis 采集跟对应中间件那台机器。</p>
            <p style="margin:0 0 0.35rem"><strong>采集范围</strong></p>
            <el-checkbox
              v-for="n in siteForm.nodes"
              :key="n.ip"
              :model-value="(siteForm.monitor.agents || siteForm.nodes.map((x) => x.ip)).includes(n.ip)"
              @change="(on) => toggleMonitorAgent(n.ip, on)"
            >{{ n.name }}（{{ n.ip }}）</el-checkbox>
            <p style="margin:0.75rem 0 0.35rem"><strong>组件采集</strong></p>
            <el-checkbox
              v-for="name in ['kafka', 'mysql', 'pgsql', 'redis']"
              :key="name"
              :model-value="(siteForm.monitor.exporters || defaultMonitorExporters()).includes(name)"
              @change="(on) => toggleMonitorExporter(name, on)"
            >{{ name }}</el-checkbox>
            <div class="actions" style="margin-top:0.75rem">
              <el-button
                type="primary"
                :class="{ 'btn-active': activeJobKey === 'monitor-deploy' }"
                :disabled="busy || !fieldPaths.platformRoot || !isServiceEnabled('monitor')"
                @click="runMonitorDeploy"
              >{{ activeJobKey === 'monitor-deploy' ? '部署中…' : '部署监控' }}</el-button>
            </div>
            </el-collapse-item>
          </el-collapse>
          <JobLogBox :lines="jobLogs" :running="!!activeJobKey" />
          <div class="actions">
            <el-button plain @click="goToStep(3)">返回</el-button>
            <el-button type="primary" @click="completeFieldStep('business', 5)" :disabled="!fieldStepDone.middleware">
              平台业务就绪，进入 ⑥ 市政/模型
            </el-button>
          </div>
        </div>
</template>
