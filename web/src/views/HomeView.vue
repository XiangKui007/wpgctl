<script>
/** 首页：入口、当前模式路径，以及本机 Docker 的现场注意点。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'HomeView',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="home">
      <div class="hero">
        <div class="hero-copy">
          <h1>WPG<em>CTL</em></h1>
          <p class="lead">{{ heroLead }}</p>
          <div class="cta-row">
            <el-button type="primary" @click="goDeployEntry">{{ isLocalDocker ? '打开部署向导' : '准备环境' }}</el-button>
            <el-button v-if="settings.advancedMode" type="primary" plain @click="openFetch">打开包中心</el-button>
          </div>
        </div>
        <div class="hero-visual">
          <div class="wave-panel">
            <div class="meta">
              <strong>{{ siteName }}</strong>
              <span>{{ siteCode }} · {{ isLocalDocker ? '本机联调' : '现场交付' }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="home-rail">
        <div class="home-flags" v-if="dockerOk === false || (settings.advancedMode && (packagesCount === 0 || !latestVersion))">
          <el-alert
            v-if="dockerOk === false"
            type="warning"
            show-icon
            :closable="false"
            title="Docker 未就绪"
            :description="isLocalDocker ? '请先启动 Docker Desktop。' : '请确认现场 Docker 已运行（向导步骤 ② 可离线安装）。'"
          />
          <template v-if="settings.advancedMode">
            <el-alert v-if="packagesCount === 0" type="info" show-icon :closable="false" title="尚无交付包" description="请先到包中心拉取或导入。">
              <el-button type="primary" plain size="small" @click="openFetch">去包中心</el-button>
            </el-alert>
            <el-alert v-if="!latestVersion" type="info" show-icon :closable="false" title="尚未首次部署" description="完成向导后可使用升级。" />
          </template>
        </div>

        <div class="mode-bar">
          <div>
            <h2>{{ isLocalDocker ? '本机 Docker 联调' : 'Linux 现场交付' }}</h2>
            <p>{{ platformHint }}</p>
          </div>
          <el-switch :model-value="isLocalDocker" active-text="本机 Docker" @change="onLocalDockerToggle" />
        </div>

        <ol class="home-sop" :aria-label="isLocalDocker ? '本机联调步骤' : '现场交付步骤'">
          <template v-if="!isLocalDocker">
            <li>
              <span class="home-sop-idx">1</span>
              <div>
                <strong>放到主控机</strong>
                <p>把 <code>wpgctl</code> 拷到现场 Linux，工作簿根默认 <code>/workspace</code>。</p>
              </div>
            </li>
            <li>
              <span class="home-sop-idx">2</span>
              <div>
                <strong>准备安装包</strong>
                <p>middleware / platform 等目录放到约定路径；缺包时再开「一键部署」用包中心。</p>
              </div>
            </li>
            <li>
              <span class="home-sop-idx">3</span>
              <div>
                <strong>按向导交付</strong>
                <p>① 节点到 ⑧ 验收在向导里完成，不用记命令。</p>
              </div>
            </li>
          </template>
          <template v-else>
            <li>
              <span class="home-sop-idx">1</span>
              <div>
                <strong>启动 Desktop</strong>
                <p>安装并启动 Docker Desktop，建议 WSL2 后端。</p>
              </div>
            </li>
            <li>
              <span class="home-sop-idx">2</span>
              <div>
                <strong>用本机路径</strong>
                <p>site 里 paths 用盘符路径，中间件可先填 <code>127.0.0.1</code>。</p>
              </div>
            </li>
            <li>
              <span class="home-sop-idx">3</span>
              <div>
                <strong>跑通向导</strong>
                <p>打开部署向导做体检 → 初始化 → 部署。Desktop 不支持 <code>network_mode: host</code>，相关服务走 bridge 端口映射。</p>
              </div>
            </li>
          </template>
        </ol>
        <p v-if="settings.advancedMode" class="home-sop-note muted">
          日常 1～3 个服务补丁走顶栏「升级」；失败会回滚镜像，SQL 需人工评估。
        </p>
      </div>
    </section>

</template>
