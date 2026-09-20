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
    WizardLocalPrecheck: defineAsyncComponent(() => import('./wizard/WizardLocalPrecheck.vue')),
    WizardLocalInit: defineAsyncComponent(() => import('./wizard/WizardLocalInit.vue')),
    WizardLocalDeploy: defineAsyncComponent(() => import('./wizard/WizardLocalDeploy.vue')),
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
          <p>
            <template v-if="isLocalDocker">本机 Docker 联调（manifest 一键流程）：请先启动 Docker Desktop；paths 使用本机盘符路径。红色体检项会阻断后续动作。</template>
            <template v-else>Linux 现场部署：默认按多机规划；paths 使用 Linux 路径。{{ isMultiNode ? '分配到从机的模块会自动通过 SSH 分发，只需在主控机点按钮。' : '' }}</template>
          </p>
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

      <div class="surface">
        <p v-if="siteError" class="wizard-alert muted">{{ siteError }}</p>

        <!-- 多机：② ~ ⑥ 步公用的 SSH 分发面板（⑦ Nginx 不再展示文件同步） -->
        <details v-if="isMultiNode && wizardStep >= 1 && wizardStep <= 5" class="ssh-panel" :open="sshPanelOpen">
          <summary @click.prevent="sshPanelOpen = !sshPanelOpen">
            <strong>SSH 分发到从机</strong>
            <span class="muted">
              · 分配到其他机器的模块会自动通过 SSH 在目标机执行（上传 wpgctl / site.yaml / 缺失的模块目录）
            </span>
            <span class="badge" :class="sshCredsReady ? 'green' : 'yellow'" style="margin-left:auto">
              {{ sshCredsReady ? '凭据已填' : '未填凭据' }}
            </span>
          </summary>
          <div class="field-grid" style="margin-top:0.6rem">
            <div class="field">
              <label>从机 SSH 密码（仅本次会话，不落盘）</label>
              <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各从机相同密码时填写" show-password />
            </div>
            <div class="field">
              <label>或 SSH 私钥路径（主控机上）</label>
              <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
            </div>
            <div class="field full">
              <el-checkbox v-model="remoteSync.syncFiles">目标机缺少模块目录时自动上传（同路径落盘，跳过 data/logs；非 Nginx 还跳过 html/frontend/dist，已存在文件不重传）</el-checkbox>
              <el-checkbox v-model="remoteSync.forceSync">强制重新同步（覆盖目标机已有目录）</el-checkbox>
            </div>
            <p class="hint-banner full" style="margin:0">
              从机账号非 root 时自动 <code>sudo</code>；从机列表：
              <span v-for="(n, i) in siteForm.nodes.slice(1)" :key="i" class="node-target">{{ n.name }} ({{ n.sshUser || 'root' }}@{{ n.ip }})</span>
            </p>
          </div>
        </details>

        <!-- ⑦ Nginx / ⑧ 验收：仅 SSH 凭据（验收时汇总从机容器） -->
        <details v-if="isMultiNode && (wizardStep === 6 || wizardStep === 7)" class="ssh-panel" :open="sshPanelOpen">
          <summary @click.prevent="sshPanelOpen = !sshPanelOpen">
            <strong>{{ wizardStep === 7 ? 'SSH 凭据（验收从机容器）' : 'SSH 凭据（从机部署 Nginx）' }}</strong>
            <span class="muted">· {{ wizardStep === 7 ? '用于汇总各从机 docker ps' : '不自动同步 conf；请先在本机编辑保存后再部署' }}</span>
            <span class="badge" :class="sshCredsReady ? 'green' : 'yellow'" style="margin-left:auto">
              {{ sshCredsReady ? '凭据已填' : '未填凭据' }}
            </span>
          </summary>
          <div class="field-grid" style="margin-top:0.6rem">
            <div class="field">
              <label>从机 SSH 密码（仅本次会话，不落盘）</label>
              <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各从机相同密码时填写" show-password />
            </div>
            <div class="field">
              <label>或 SSH 私钥路径（主控机上）</label>
              <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
            </div>
          </div>
        </details>
        <WizardSiteStep v-if="wizardStep === 0" />
        <WizardFieldDocker v-else-if="!isLocalDocker && wizardStep === 1" />
        <WizardFieldDatabase v-else-if="!isLocalDocker && wizardStep === 2" />
        <WizardFieldMiddleware v-else-if="!isLocalDocker && wizardStep === 3" />
        <WizardFieldBusiness v-else-if="!isLocalDocker && wizardStep === 4" />
        <WizardFieldStandalone v-else-if="!isLocalDocker && wizardStep === 5" />
        <WizardFieldNginx v-else-if="!isLocalDocker && wizardStep === 6" />
        <WizardFieldVerify v-else-if="!isLocalDocker && wizardStep === 7" />
        <WizardLocalPrecheck v-else-if="isLocalDocker && wizardStep === 1" />
        <WizardLocalInit v-else-if="isLocalDocker && wizardStep === 2" />
        <WizardLocalDeploy v-else-if="isLocalDocker && wizardStep === 3" />
      </div>
    </section>
</template>
