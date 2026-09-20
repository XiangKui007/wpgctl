<script>
/** 现场向导 ⑦：Nginx conf 与部署。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldNginx',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 7 / 8</strong> — 前端静态只部署到<strong>中间件所在机器</strong>，由 Nginx 转发，不随业务服务分发到其它节点。先编辑并保存 <code>http-web-8877.conf</code>，再点「部署」完成 load / 解压 html / compose 启动。
            <span v-if="isMultiNode" class="node-target">@ {{ moduleTargetLabel('nginx', 'middleware') }}</span>
          </div>
          <div class="field full" style="margin-bottom:0.75rem">
            <label class="req">nginx 模块目录</label>
            <div class="path-row">
              <el-input v-model="fieldPaths.nginxDir" :placeholder="moduleDir('middleware', 'nginx') || '.../middleware/nginx'" />
              <el-button type="primary" plain size="small" @click="openPicker('nginxDir', 'dir')">浏览</el-button>
            </div>
            <p class="muted" style="margin:0.35rem 0 0;font-size:0.85rem">
              配置文件：<code>{{ nginxWebConfPath }}</code>（会自动查找 conf/conf.d，以及多层 middleware/middleware/nginx）<br />
              前端静态：<code>{{ nginxHtmlPath }}</code>
            </p>
          </div>
          <div class="actions" :class="{ busy }" style="margin-bottom:0.75rem">
            <el-button
              type="primary"
              plain
              size="small"
              @click="openNginxConfEditor"
              :disabled="busy || !fieldPaths.nginxDir"
            >编辑 conf</el-button>
            <el-button type="primary"
              :class="{ 'btn-active': activeJobKey === 'nginx-patch' }"
              @click="runNginxDeploy"
              :disabled="busy || !fieldPaths.nginxDir || !isServiceEnabled('nginx')"
            >
              {{ activeJobKey === 'nginx-patch' ? '部署中…' : '部署' }}
            </el-button>
            <el-button plain @click="goToStep(5)">返回</el-button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="'ngx'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="fieldStepDone.nginx || nginxPatchDone">
            <el-button type="primary" @click="completeFieldStep('nginx', 7)">Nginx 就绪，进入 ⑧ 验收</el-button>
          </div>
        </div>
</template>
