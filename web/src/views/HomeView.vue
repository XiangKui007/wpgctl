<script>
/** 首页：入口与现场交付路径。 */
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
            <el-button type="primary" @click="goDeployEntry">准备环境</el-button>
            <el-button v-if="settings.advancedMode" type="primary" plain @click="openFetch">打开包中心</el-button>
          </div>
        </div>
        <div class="hero-visual">
          <div class="wave-panel">
            <div class="meta">
              <strong>{{ siteName }}</strong>
              <span>{{ siteCode }}</span>
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
            description="请确认现场 Docker 已运行（向导步骤 ② 可离线安装）。"
          />
          <template v-if="settings.advancedMode">
            <el-alert v-if="packagesCount === 0" type="info" show-icon :closable="false" title="尚无交付包" description="请先到包中心拉取或导入。">
              <el-button type="primary" plain size="small" @click="openFetch">去包中心</el-button>
            </el-alert>
            <el-alert v-if="!latestVersion" type="info" show-icon :closable="false" title="尚未首次部署" description="完成向导后可使用升级。" />
          </template>
        </div>

        <header class="home-sop-head">
          <h2>开始交付</h2>
          <p>按下面三步，完成水厂项目现场部署</p>
        </header>
        <ol class="home-sop" aria-label="现场交付步骤">
            <li>
              <span class="home-sop-idx">1</span>
              <div>
                <strong>放到主控机</strong>
                <p>将 wpgctl 二进制上传至现场 Linux 主控机，默认工作目录 <code>/workspace</code></p>
              </div>
            </li>
            <li>
              <span class="home-sop-idx">2</span>
              <div>
                <strong>准备安装包</strong>
                <p>将基础包、版本包、补丁包放置到约定目录</p>
              </div>
            </li>
            <li>
              <span class="home-sop-idx">3</span>
              <div>
                <strong>按向导交付</strong>
                <p>跟随向导完成节点配置、部署与验收。熟练后可使用【一键部署】（二期功能）</p>
              </div>
            </li>
        </ol>
        <p v-if="settings.advancedMode" class="home-sop-note muted">
          日常 1～3 个服务补丁走顶栏「升级」；失败会回滚镜像，SQL 需人工评估。
        </p>
      </div>
    </section>

</template>
