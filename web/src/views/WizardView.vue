<script>
/** 部署向导壳：步骤条、SSH 分发；步骤页按需懒加载，单独成分包。 */
import { defineAsyncComponent } from 'vue'
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardView',
  components: {
    WizardSiteStep: defineAsyncComponent(() => import('./wizard/WizardSiteStep.vue')),
    WizardFieldDocker: defineAsyncComponent(() => import('./wizard/WizardFieldDocker.vue')),
    WizardFieldDatabase: defineAsyncComponent(() => import('./wizard/WizardFieldDatabase.vue')),
    WizardFieldMiddleware: defineAsyncComponent(() => import('./wizard/WizardFieldMiddleware.vue')),
    WizardFieldBusiness: defineAsyncComponent(() => import('./wizard/WizardFieldBusiness.vue')),
    WizardFieldStandalone: defineAsyncComponent(() => import('./wizard/WizardFieldStandalone.vue')),
    WizardFieldNginx: defineAsyncComponent(() => import('./wizard/WizardFieldNginx.vue')),
    WizardFieldVerify: defineAsyncComponent(() => import('./wizard/WizardFieldVerify.vue')),
  },
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel panel-wizard">
      <div class="panel-head">
        <div>
          <h2>部署向导</h2>
          <p>Linux 现场部署。</p>
        </div>
      </div>

      <div class="steps">
        <div
          v-for="(s, i) in wizardStepDefs"
          :key="s.key"
          class="step"
          role="tab"
          :class="stepTabClass(i)"
          :title="canGoToStep(i) ? s.title : '请先完成上一步'"
          :aria-selected="wizardStep === i"
          @click="goToStep(i)"
        >
          <div class="idx">{{ s.idx }}</div>
          <div class="title">{{ s.title }}</div>
        </div>
      </div>

      <!-- 短操作（保存 / 建目录 / 探测）只遮当前面板；长任务不遮，看日志框 -->
      <div class="surface" v-loading="panelBusy" :element-loading-text="busyText">
        <p v-if="siteError" class="wizard-alert muted">{{ siteError }}</p>

        <!-- 多机：② ~ ⑥ 步公用的 SSH 分发面板（⑦ Nginx 不再展示文件同步） -->
        <details v-if="isMultiNode && wizardStep >= 1 && wizardStep <= 5" class="ssh-panel" :open="sshPanelOpen">
          <summary @click.prevent="sshPanelOpen = !sshPanelOpen">
            <strong>SSH 分发</strong>
            <span class="muted">· 从机模块经 SSH 执行</span>
            <span class="badge" :class="sshCredsReady ? 'green' : 'yellow'" style="margin-left:auto">
              {{ sshCredsReady ? '凭据已填' : '未填凭据' }}
            </span>
          </summary>
          <div class="field-grid" style="margin-top:0.6rem">
            <div class="field">
              <label>未单独填写的机器使用此 SSH 密码</label>
              <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各从机相同密码时填写" show-password />
            </div>
            <div class="field">
              <label>或 SSH 私钥路径（主控机上）</label>
              <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
            </div>
            <div class="field full">
              <el-checkbox v-model="remoteSync.syncFiles">缺少目录时自动上传（跳过 data/logs 与前端静态）</el-checkbox>
              <el-checkbox v-model="remoteSync.forceSync">强制覆盖已有目录</el-checkbox>
            </div>
            <p class="hint-banner full" style="margin:0">
              从机：
              <span v-for="(n, i) in siteForm.nodes.slice(1)" :key="i" class="node-target">{{ n.name }} ({{ n.sshUser || 'root' }}@{{ n.ip }})</span>
            </p>
          </div>
        </details>

        <!-- ⑧ 验收：SSH 凭据用于汇总从机容器；⑦ Nginx 只在主控机部署前端，不经 SSH -->
        <details v-if="isMultiNode && wizardStep === 7" class="ssh-panel" :open="sshPanelOpen">
          <summary @click.prevent="sshPanelOpen = !sshPanelOpen">
            <strong>SSH 凭据</strong>
            <span class="muted">· 验收从机容器</span>
            <span class="badge" :class="sshCredsReady ? 'green' : 'yellow'" style="margin-left:auto">
              {{ sshCredsReady ? '凭据已填' : '未填凭据' }}
            </span>
          </summary>
          <div class="field-grid" style="margin-top:0.6rem">
            <div class="field">
              <label>未单独填写的机器使用此 SSH 密码</label>
              <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各从机相同密码时填写" show-password />
            </div>
            <div class="field">
              <label>或 SSH 私钥路径（主控机上）</label>
              <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
            </div>
          </div>
        </details>
        <WizardSiteStep v-if="wizardStep === 0" />
        <WizardFieldDocker v-else-if="wizardStep === 1" />
        <WizardFieldDatabase v-else-if="wizardStep === 2" />
        <WizardFieldMiddleware v-else-if="wizardStep === 3" />
        <WizardFieldBusiness v-else-if="wizardStep === 4" />
        <WizardFieldStandalone v-else-if="wizardStep === 5" />
        <WizardFieldNginx v-else-if="wizardStep === 6" />
        <WizardFieldVerify v-else-if="wizardStep === 7" />
      </div>
    </section>
</template>
