<template>
  <div class="app-shell" :class="isLocalDocker ? 'mode-local' : 'mode-field'">
    <header class="topbar">
      <div class="chrome">
        <div class="brand-mark">
          <span class="mark" aria-hidden="true"></span>
          <div class="brand-text">
            <strong>WPGCTL</strong>
            <span>{{ isLocalDocker ? '本机 Docker 联调' : 'Linux 现场交付' }}</span>
          </div>
        </div>
        <div class="topbar-tools">
          <div class="env-chip" v-if="runtimeOS" :class="{ warn: dockerOk === false }">{{ envChipText }}</div>
          <label class="settings-item">
            <span>操作者</span>
            <input type="text" v-model="settings.operator" @change="saveSettings" placeholder="署名" />
          </label>
          <label class="local-switch" :title="isLocalDocker ? '关闭后回到 Linux 现场交付' : '开启后切换为本机 Docker 联调'">
            <input type="checkbox" :checked="isLocalDocker" @change="onLocalDockerToggle" />
            <span class="switch-track" aria-hidden="true"></span>
            <span class="switch-text">本机 Docker</span>
          </label>
          <label class="privacy-toggle">
            <input type="checkbox" v-model="settings.privacyMode" @change="saveSettings" />
            <span>投屏</span>
          </label>
        </div>
      </div>
      <div class="nav-row">
        <nav class="nav">
          <button :class="{ active: view === 'home' }" @click="view = 'home'">首页</button>
          <button :class="{ active: view === 'wizard' }" @click="goWizard">交付向导</button>
          <button :class="{ active: view === 'fetch' }" @click="openFetch">包中心</button>
          <button
            :class="{ active: view === 'upgrade', disabled: !latestVersion }"
            :title="!latestVersion ? '请先完成首次部署' : ''"
            :disabled="!latestVersion"
            @click="openUpgrade"
          >升级</button>
          <button :class="{ active: view === 'status' }" @click="openStatus">状态</button>
          <button :class="{ active: view === 'history' }" @click="openHistory">交付单</button>
          <button :class="{ active: view === 'guide' }" @click="view = 'guide'">怎么用</button>
          <button :class="{ active: view === 'logs' }" @click="view = 'logs'">日志</button>
          <button :class="{ active: view === 'report' }" @click="openReport()">验收报告</button>
        </nav>
        <div class="nav-tools">
          <button type="button" class="nav-quiet" @click="downloadDiag" :disabled="busy">诊断包</button>
          <a class="nav-quiet" href="/api/deliveries/export">导出摘要</a>
        </div>
      </div>
    </header>

    <!-- HOME -->
    <section v-if="view === 'home'" class="home">
      <div class="hero">
        <div class="hero-copy">
          <h1>WPG<em>CTL</em></h1>
          <p class="lead">{{ heroLead }}</p>
          <div class="cta-row">
            <button class="btn btn-primary" @click="goWizard">打开交付向导</button>
            <button class="btn btn-ghost" @click="openFetch">打开包中心</button>
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
        <div class="home-flags" v-if="dockerOk === false || packagesCount === 0 || !latestVersion">
          <div v-if="dockerOk === false" class="flag warn">
            <strong>Docker 未就绪</strong>
            <span v-if="isLocalDocker">请先启动 Docker Desktop。</span>
            <span v-else>请确认现场 Docker 已运行。</span>
          </div>
          <div v-if="packagesCount === 0" class="flag">
            <strong>尚无交付包</strong>
            <span>请先到包中心拉取或导入。</span>
            <button class="btn btn-ghost btn-sm" @click="openFetch">去包中心</button>
          </div>
          <div v-if="!latestVersion" class="flag">
            <strong>尚未首次部署</strong>
            <span>完成向导后可使用升级。</span>
          </div>
        </div>

        <div class="mode-bar">
          <div>
            <h2>{{ isLocalDocker ? '本机 Docker 联调' : 'Linux 现场交付' }}</h2>
            <p>{{ platformHint }}</p>
          </div>
          <label class="local-switch">
            <input type="checkbox" :checked="isLocalDocker" @change="onLocalDockerToggle" />
            <span class="switch-track" aria-hidden="true"></span>
            <span class="switch-text">本机 Docker</span>
          </label>
        </div>
      </div>
    </section>

    <!-- GUIDE -->
    <section v-else-if="view === 'guide'" class="panel">
      <div class="panel-head">
        <div>
          <h2>使用说明</h2>
          <p>按场景选命令；日常推荐用页面向导。</p>
        </div>
        <button class="btn btn-ghost" @click="view = 'home'">返回首页</button>
      </div>
      <div class="surface guide-blocks">
        <div v-if="!isLocalDocker">
          <h3>现场 Linux 主控机（当前）</h3>
          <ol class="guide-ol">
            <li>把 <code>wpgctl</code> 与 <code>site.yaml</code> 拷到主控机</li>
            <li>准备 base / release 包（或包中心拉取）</li>
            <li>向导或命令：<code>precheck → init → deploy</code></li>
          </ol>
        </div>
        <div v-else>
          <h3>本机 Docker 联调（当前）</h3>
          <ol class="guide-ol">
            <li>安装并启动 Docker Desktop（建议 WSL2 后端）</li>
            <li>编辑 <code>site.yaml</code>：中间件可先用 <code>127.0.0.1</code>，paths 用本机盘符路径</li>
            <li><code>wpgctl ui --site site.yaml</code> → 向导：precheck → init → deploy</li>
            <li>注意：Desktop 不支持 Linux 的 <code>network_mode: host</code>，相关服务需 bridge 端口映射</li>
          </ol>
        </div>
        <div>
          <h3>日常升级 1~3 个服务</h3>
          <ol class="guide-ol">
            <li>拿到 patch 包目录</li>
            <li><code>wpgctl upgrade ./wpg-patch-4.0.3 --site site.yaml --yes</code></li>
            <li>失败会自动回滚镜像；SQL 需人工评估</li>
          </ol>
        </div>
        <div class="actions">
          <button class="btn btn-primary" @click="goWizard">去交付向导</button>
        </div>
      </div>
    </section>

    <!-- WIZARD -->
    <section v-else-if="view === 'wizard'" class="panel">
      <div class="panel-head">
        <div>
          <h2>交付向导</h2>
          <p>
            {{ isLocalDocker ? '本机 Docker 联调：请先启动 Docker Desktop；paths 使用本机盘符路径。' : 'Linux 现场交付：请确认主控机 Docker 可用；paths 使用 Linux 路径。' }}
            红色体检项会阻断后续动作。
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
        <div v-if="fileEditor.path && fileEditor.path.endsWith('.env')" class="file-editor" style="margin-bottom:1rem">
          <div class="file-editor-head">
            <span>编辑 <code>{{ fileEditor.path }}</code></span>
            <span>
              <button class="btn btn-ghost btn-sm" @click="saveTextFile()" :disabled="busy || !fileEditor.text">保存</button>
              <button class="btn btn-ghost btn-sm" @click="fileEditor.path = ''">关闭</button>
            </span>
          </div>
          <textarea v-model="fileEditor.text" spellcheck="false"></textarea>
          <p v-if="fileEditor.msg" class="muted" style="padding:0.35rem 0.75rem;margin:0;font-size:0.85rem">{{ fileEditor.msg }}</p>
        </div>
        <p v-if="siteError" class="wizard-alert muted">{{ siteError }}</p>
        <!-- step 0 -->
        <div v-if="wizardStep === 0">
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <p class="hint-banner">
            <template v-if="isLocalDocker">可用「表单」改常用项，或切到「YAML」精细编辑。</template>
            <template v-else>
              <strong>Linux 现场 SOP：</strong>先配好本节点各中间件 IP（密码用默认），再按顺序执行各步；顶栏 Tab 可回看已完成步骤。
            </template>
            保存会写入 {{ sitePath || 'site.yaml' }}。
          </p>

          <div class="seg" style="margin:0 0 1.1rem" role="group" aria-label="编辑方式">
            <button type="button" :class="{ on: siteEditMode === 'form' }" @click="switchSiteMode('form')">表单</button>
            <button type="button" :class="{ on: siteEditMode === 'yaml' }" @click="switchSiteMode('yaml')">YAML</button>
          </div>

          <div v-if="siteEditMode === 'form'" class="field-grid">
            <div class="field">
              <label>项目名称</label>
              <input v-model="siteForm.site.name" placeholder="水厂/项目显示名" />
            </div>
            <div class="field">
              <label>项目编码</label>
              <input v-model="siteForm.site.code" placeholder="节点 site.yaml 标识" />
            </div>
            <div class="field full">
              <label>业务模块</label>
              <div class="module-grid">
                <label
                  v-for="m in moduleDefs"
                  :key="m.id"
                  class="module-card"
                  :class="{ selected: selectedModules[m.id] }"
                >
                  <input type="checkbox" v-model="selectedModules[m.id]" />
                  <span>{{ m.label }}</span>
                </label>
              </div>
            </div>
            <div class="field">
              <label>Nacos Host</label>
              <input v-model="siteForm.middleware.nacos.host" />
            </div>
            <div class="field">
              <label>Nacos 用户 / 密码</label>
              <div class="path-row credential-row">
                <input v-model="siteForm.middleware.nacos.username" placeholder="nacos" />
                <div class="password-wrap">
                  <input
                    v-model="siteForm.middleware.nacos.password"
                    :type="passwordVisible.nacos ? 'text' : 'password'"
                    :placeholder="settings.privacyMode ? '******' : '标准密码（已预设）'"
                    autocomplete="new-password"
                  />
                  <button
                    type="button"
                    class="password-toggle"
                    :title="passwordVisible.nacos ? '隐藏密码' : '显示密码'"
                    :aria-label="passwordVisible.nacos ? '隐藏密码' : '显示密码'"
                    @click="passwordVisible.nacos = !passwordVisible.nacos"
                  >
                    <svg v-if="passwordVisible.nacos" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
                      <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                      <line x1="1" y1="1" x2="23" y2="23" />
                    </svg>
                    <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                      <circle cx="12" cy="12" r="3" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
            <div class="field full">
              <label class="checkbox-label">
                <input v-model="siteForm.middleware.mysql.disabled" type="checkbox" />
                本版本不依赖 MySQL（仅 PgSQL，跳过 MySQL 部署与连通检查）
              </label>
            </div>
            <template v-if="!siteForm.middleware.mysql.disabled">
              <div class="field">
                <label>MySQL Host</label>
                <input v-model="siteForm.middleware.mysql.host" />
              </div>
              <div class="field">
                <label>MySQL 用户 / 密码</label>
                <div class="path-row credential-row">
                  <input v-model="siteForm.middleware.mysql.user" placeholder="wpg" />
                  <div class="password-wrap">
                    <input
                      v-model="siteForm.middleware.mysql.password"
                      :type="passwordVisible.mysql ? 'text' : 'password'"
                      :placeholder="settings.privacyMode ? '******' : '标准密码（已预设）'"
                      autocomplete="new-password"
                    />
                    <button
                      type="button"
                      class="password-toggle"
                      :title="passwordVisible.mysql ? '隐藏密码' : '显示密码'"
                      :aria-label="passwordVisible.mysql ? '隐藏密码' : '显示密码'"
                      @click="passwordVisible.mysql = !passwordVisible.mysql"
                    >
                      <svg v-if="passwordVisible.mysql" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                        <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
                        <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                        <line x1="1" y1="1" x2="23" y2="23" />
                      </svg>
                      <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                        <circle cx="12" cy="12" r="3" />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            </template>
            <div class="field">
              <label>Redis Host</label>
              <input v-model="siteForm.middleware.redis.host" />
            </div>
            <div class="field">
              <label>Redis 密码</label>
              <div class="password-wrap">
                <input
                  v-model="siteForm.middleware.redis.password"
                  :type="passwordVisible.redis ? 'text' : 'password'"
                  :placeholder="settings.privacyMode ? '******' : '标准密码（已预设）'"
                  autocomplete="new-password"
                />
                <button
                  type="button"
                  class="password-toggle"
                  :title="passwordVisible.redis ? '隐藏密码' : '显示密码'"
                  :aria-label="passwordVisible.redis ? '隐藏密码' : '显示密码'"
                  @click="passwordVisible.redis = !passwordVisible.redis"
                >
                  <svg v-if="passwordVisible.redis" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
                    <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                    <line x1="1" y1="1" x2="23" y2="23" />
                  </svg>
                  <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                </button>
              </div>
            </div>
            <div class="field">
              <label>PgSQL Host</label>
              <input v-model="siteForm.middleware.pgsql.host" />
            </div>
            <div class="field">
              <label>PgSQL 用户 / 密码</label>
              <div class="path-row credential-row">
                <input v-model="siteForm.middleware.pgsql.user" placeholder="wpg" />
                <div class="password-wrap">
                  <input
                    v-model="siteForm.middleware.pgsql.password"
                    :type="passwordVisible.pgsql ? 'text' : 'password'"
                    :placeholder="settings.privacyMode ? '******' : '标准密码（已预设）'"
                    autocomplete="new-password"
                  />
                  <button
                    type="button"
                    class="password-toggle"
                    :title="passwordVisible.pgsql ? '隐藏密码' : '显示密码'"
                    :aria-label="passwordVisible.pgsql ? '隐藏密码' : '显示密码'"
                    @click="passwordVisible.pgsql = !passwordVisible.pgsql"
                  >
                    <svg v-if="passwordVisible.pgsql" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
                      <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                      <line x1="1" y1="1" x2="23" y2="23" />
                    </svg>
                    <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                      <circle cx="12" cy="12" r="3" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
            <div class="field">
              <label>Kafka Host</label>
              <input v-model="siteForm.middleware.kafka.host" />
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <label>部署拓扑</label>
              <div class="seg" role="group" aria-label="部署拓扑">
                <button type="button" :class="{ on: deployTopology === 'single' }" @click="setDeployTopology('single')">单机</button>
                <button type="button" :class="{ on: deployTopology === 'multi' }" @click="setDeployTopology('multi')">多机</button>
              </div>
              <p v-if="deployTopology === 'single'" class="muted" style="margin:0.45rem 0 0;font-size:0.85rem">
                所有服务部署在一台机器；下方 IP 写入 <code>nodes[0]</code>。
              </p>
              <p v-else class="muted" style="margin:0.45rem 0 0;font-size:0.85rem">
                按角色拆分机器（如 db-node + app-node）；Init 会向从机 SSH 分发；各步请在对应机器上执行模块部署。
              </p>
            </div>
            <div v-if="!isLocalDocker && deployTopology === 'single'" class="field">
              <label>主节点 IP</label>
              <input v-model="siteForm.nodes[0].ip" placeholder="现场主控机 IP" />
            </div>
            <div v-if="!isLocalDocker && deployTopology === 'multi'" class="field full">
              <div class="host-fill-bar" style="margin-bottom:0.5rem">
                <button type="button" class="btn btn-ghost btn-sm" @click="applyDualNodeTemplate">套用双机模板</button>
                <button type="button" class="btn btn-ghost btn-sm" @click="addNode">添加机器</button>
                <button type="button" class="btn btn-ghost btn-sm" @click="applyMiddlewareFromRoles">按角色填充中间件 Host</button>
              </div>
              <div class="node-table-wrap">
                <table class="node-table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>IP</th>
                      <th>SSH 用户</th>
                      <th>端口</th>
                      <th>角色</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(n, idx) in siteForm.nodes" :key="idx">
                      <td><input v-model="n.name" placeholder="db-node" /></td>
                      <td><input v-model="n.ip" placeholder="10.0.0.1" /></td>
                      <td><input v-model="n.sshUser" placeholder="root" /></td>
                      <td style="width:4.5rem"><input v-model.number="n.sshPort" type="number" min="1" max="65535" /></td>
                      <td>
                        <div class="role-chips">
                          <label
                            v-for="r in NODE_ROLE_OPTIONS"
                            :key="r.id"
                            class="role-chip"
                            :class="{ on: n.roles.includes(r.id) }"
                          >
                            <input type="checkbox" :checked="n.roles.includes(r.id)" @change="toggleNodeRole(n, r.id)" hidden />
                            {{ r.label }}
                          </label>
                        </div>
                      </td>
                      <td>
                        <button type="button" class="btn btn-ghost btn-sm" @click="removeNode(idx)" :disabled="siteForm.nodes.length <= 1">删除</button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="field-grid" style="margin-top:0.75rem">
                <div class="field">
                  <label>SSH 密码（仅本次 Init，不写入 site.yaml）</label>
                  <input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各节点相同密码时使用" />
                </div>
                <div class="field">
                  <label>或 SSH 私钥路径</label>
                  <input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
                </div>
              </div>
            </div>
            <div v-if="isLocalDocker" class="field">
              <label>本机节点 IP</label>
              <input v-model="siteForm.nodes[0].ip" placeholder="127.0.0.1" />
            </div>
            <div class="field full">
              <div class="host-fill-bar">
                <button type="button" class="btn btn-ghost btn-sm" @click="fillHostsAuto" :disabled="busy">
                  自动获取本机 Host
                </button>
                <button type="button" class="btn btn-ghost btn-sm" @click="applyDefaultCreds(true)" :disabled="busy">
                  填充默认账号密码
                </button>
                <select
                  v-if="hostOptions.length > 1"
                  v-model="selectedHost"
                  class="host-select"
                  @change="applySelectedHost"
                >
                  <option v-for="ip in hostOptions" :key="ip" :value="ip">{{ ip }}</option>
                </select>
                <span v-if="hostFillMsg" class="muted host-fill-msg">{{ hostFillMsg }}</span>
              </div>
              <p class="hint-banner" style="margin:0.45rem 0 0">
                已预设公司标准账号密码（Nacos namespace <code>intergrate</code>，Kafka 无密码）。现场与标准包一致时不用改。
              </p>
            </div>
            <div class="field full">
              <label>workspace 路径</label>
              <div class="path-row">
                <input
                  v-model="siteForm.paths.workspace"
                  :placeholder="isLocalDocker ? 'D:/workspace/waterwork' : '/workspace/waterwork'"
                />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('pathsWorkspace', 'dir')">浏览</button>
              </div>
            </div>
            <div class="field full">
              <label>nginxHtml（前端静态，= nginx/html）</label>
              <div class="path-row">
                <input
                  v-model="siteForm.paths.nginxHtml"
                  :placeholder="nginxHtmlPath || '/workspace/middleware/nginx/html'"
                />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('pathsNginxHtml', 'dir')">浏览</button>
              </div>
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <button type="button" class="btn btn-ghost btn-sm" @click="applyPathsFromMiddleware">
                按 middleware 根目录填充 paths
              </button>
            </div>
          </div>

          <div v-else class="field full" style="margin-bottom: 1rem">
            <label>site.yaml 内容（YAML 高亮编辑）</label>
            <YamlEditor v-model="siteYaml" />
          </div>
          <p v-if="siteSaveMsg" class="muted" style="color: var(--ok)">{{ siteSaveMsg }}</p>
          <div class="actions" style="margin-top: 0.5rem">
            <button class="btn btn-primary" @click="saveSite" :disabled="busy">保存 site.yaml</button>
            <button class="btn btn-ghost" @click="loadSite">重新加载</button>
          </div>

          <hr class="sep" />

          <div class="field-grid">
            <div class="field full">
              <label>Manifest 文件（体检用，可选）</label>
              <div class="path-row">
                <input v-model="form.manifest" placeholder="选择 .yaml / .yml" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('manifest', 'yaml')">浏览</button>
              </div>
            </div>
            <div class="field full">
              <label>Release 包目录</label>
              <div class="path-row">
                <input v-model="form.package" placeholder="选择含 manifest.yaml 的包目录" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('package', 'dir')">浏览</button>
              </div>
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <label>middleware 根目录</label>
              <div class="path-row">
                <input v-model="fieldPaths.middlewareRoot" placeholder="含 mysql/pgsql/redis 的目录（单层或外层均可）" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('middlewareRoot', 'dir')">浏览</button>
              </div>
              <p class="hint-banner" style="margin:0.35rem 0 0">
                填<strong>直接含</strong> <code>mysql</code>、<code>pgsql</code> 的那层，或其<strong>外层</strong>均可。
                例：单层 <code>.../middleware/mysql</code> 填 <code>.../middleware</code>；
                双层 <code>.../middleware/middleware/mysql</code> 填内层或外层都行。
              </p>
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <label>platform 根目录</label>
              <div class="path-row">
                <input v-model="fieldPaths.platformRoot" placeholder="含 public/device 的目录（单层或外层均可）" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('platformRoot', 'dir')">浏览</button>
              </div>
              <p class="hint-banner" style="margin:0.35rem 0 0">
                同上：单层填 <code>.../platform</code>（其下有 <code>public</code>）；双层填 <code>.../platform/platform</code> 或外层 <code>.../platform</code>。
              </p>
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <label>市政水厂包目录（可选）</label>
              <div class="path-row">
                <input
                  v-model="siteForm.paths.waterwork"
                  placeholder="如 /workspace/waterwork-4.1.1（独立包，非 platform 子目录）"
                />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('waterworkDir', 'dir')">浏览</button>
              </div>
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <label>模型服务包目录（可选）</label>
              <div class="path-row">
                <input
                  v-model="siteForm.paths.intelligentModel"
                  placeholder="如 /workspace/wpg-intelligent-model-4.1.2（独立包）"
                />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('intelligentModelDir', 'dir')">浏览</button>
              </div>
            </div>
            <div v-if="!isLocalDocker" class="field full">
              <label>Docker 离线安装目录（Linux 首次必装）</label>
              <div class="path-row">
                <input
                  v-model="form.dockerPackage"
                  placeholder="如 middleware/docker_package/docker_package"
                />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('dockerPackage', 'dir')">浏览</button>
              </div>
              <p class="hint-banner" style="margin:0.35rem 0 0">
                含 <code>offline_install_docker.sh</code>、<code>docker-*.tgz</code> 的目录；浏览时带 <code>[docker]</code> 标记。
              </p>
            </div>
            <div class="field full">
              <label>Base 包目录（可选，标准 base 或 middleware 根目录）</label>
              <div class="path-row">
                <input v-model="form.base" placeholder="标准 base 含 docker-install/；或 middleware 根目录" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('base', 'dir')">浏览</button>
              </div>
            </div>
          </div>
          <div class="actions">
            <button type="button" class="btn btn-primary" @click="nextFromSite" :disabled="busy">
              {{ isLocalDocker ? '下一步：环境体检' : '保存并进入 ② 安装 Docker' }}
            </button>
          </div>
        </div>

        <!-- Linux 现场 step 1: Docker -->
        <div v-else-if="!isLocalDocker && wizardStep === 1">
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 2 / 8</strong> — 在本机（主控）执行 Docker 离线安装；数据目录指向挂载盘 <code>…/docker_data/docker/lib</code>。
            <template v-if="isMultiNode"> 多机时 Init 还会 SSH 到从机做目录/防火墙初始化；从机 Docker 需在各机分别安装或登录后重复本步。</template>
          </div>
          <div class="field-grid" style="margin-top:0.75rem">
            <div class="field full">
              <label>Docker 离线包目录</label>
              <div class="path-row">
                <input v-model="form.dockerPackage" placeholder="middleware/docker_package/docker_package" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('dockerPackage', 'dir')">浏览</button>
              </div>
            </div>
          </div>
          <div class="actions" :class="{ busy }">
            <button class="btn btn-primary" :class="{ 'btn-active': activeJobKey === 'init' }" @click="runInit" :disabled="busy || !form.dockerPackage">
              {{ busy && activeJobKey === 'init' ? '安装中…' : '执行 Docker 安装 (Init)' }}
            </button>
            <button class="btn btn-ghost" @click="goToStep(0)">返回</button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="initDone">
            <button class="btn btn-primary" @click="completeFieldStep('docker', 2)">Docker 就绪，进入 ③ 数据库</button>
          </div>
        </div>

        <!-- Linux step 2: 数据库 -->
        <div v-else-if="!isLocalDocker && wizardStep === 2">
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 3 / 8</strong> — 有 zip 则解压，已有 .tar 则跳过解压直接 load → compose up → 确认 init SQL。
          </div>
          <div class="module-deploy-list">
            <div v-for="m in fieldDatabaseModules" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ moduleDir('database', m.name) }}</span>
                <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel('database') }}</span>
                <span v-if="fieldModuleStatus[m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus[m.name] }}</span>
              </div>
              <button
                class="btn btn-ghost btn-sm"
                :class="{ 'btn-active': activeJobKey === 'database-' + m.name }"
                @click="runModuleDeploy('database', m.name, false)"
                :disabled="busy || !fieldPaths.middlewareRoot"
              >
                {{ activeJobKey === 'database-' + m.name ? '执行中…' : '解压+Load+启动' }}
              </button>
              <button class="btn btn-ghost btn-sm" @click="openEnvEditor('database', m.name)" :disabled="busy">编辑 .env</button>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'db'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <button class="btn btn-ghost" @click="goToStep(1)">返回</button>
            <button class="btn btn-primary" @click="completeFieldStep('database', 3)" :disabled="!fieldStepDone.docker">
              数据库就绪，进入 ④ 中间件
            </button>
          </div>
        </div>

        <!-- Linux step 3: 中间件 -->
        <div v-else-if="!isLocalDocker && wizardStep === 3">
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="field full" style="margin-bottom:0.75rem">
            <label>Nacos 配置目录（部署 Nacos 后自动导入）</label>
            <div class="path-row">
              <input v-model="fieldPaths.nacosConfigDir" placeholder="nacos 配置导出目录" />
              <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('nacosConfigDir', 'dir')">浏览</button>
            </div>
          </div>
          <div class="hint-banner">
            <strong>步骤 4 / 8</strong> — 顺序：redis → kafka → nacos → …；Nacos 启动后自动导入配置；Kafka 会改 compose 内 KAFKA_ADVERTISED_LISTENERS。
          </div>
          <div class="module-deploy-list">
            <div v-for="m in fieldModules.middleware" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ moduleDir('middleware', m.name) }}</span>
                <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel('middleware') }}</span>
                <span v-if="fieldModuleStatus[m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus[m.name] }}</span>
              </div>
              <button
                class="btn btn-ghost btn-sm"
                :class="{ 'btn-active': activeJobKey === 'middleware-' + m.name }"
                @click="runModuleDeploy('middleware', m.name, false)"
                :disabled="busy || !fieldPaths.middlewareRoot"
              >
                {{ activeJobKey === 'middleware-' + m.name ? '执行中…' : (m.name === 'nacos' && fieldPaths.nacosConfigDir ? '启动+导入Nacos' : '解压+Load+启动') }}
              </button>
              <button
                v-if="m.name === 'nacos' && fieldPaths.nacosConfigDir"
                class="btn btn-ghost btn-sm"
                :class="{ 'btn-active': activeJobKey === 'nacos-import' }"
                @click="runNacosImport"
                :disabled="busy"
              >
                重新导入
              </button>
              <button class="btn btn-ghost btn-sm" @click="openEnvEditor('middleware', m.name)" :disabled="busy">编辑 .env</button>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'mw'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <button class="btn btn-ghost" @click="goToStep(2)">返回</button>
            <button class="btn btn-primary" @click="completeFieldStep('middleware', 4)" :disabled="!fieldStepDone.database">
              中间件就绪，进入 ⑤ 业务服务
            </button>
          </div>
        </div>

        <!-- Linux step 4: 业务 -->
        <div v-else-if="!isLocalDocker && wizardStep === 4">
          <div class="hint-banner">
            <strong>步骤 5 / 8</strong> — 各 platform 服务目录通常含 <code>*.tar.zip</code>，需先展开为 <code>.tar</code> 再 <code>docker load</code>，然后按 site.yaml <strong>只改 .env IP</strong> → compose up。
          </div>
          <div class="actions" style="margin-top:0;margin-bottom:0.75rem">
            <button class="btn btn-ghost btn-sm" @click="expandPlatformArchives(true)" :disabled="busy || !fieldPaths.platformRoot">
              批量解压 platform 全部 tar.zip
            </button>
            <button class="btn btn-ghost btn-sm" @click="patchAllBusinessEnv" :disabled="busy || !fieldPaths.platformRoot">
              批量更新 platform .env IP
            </button>
          </div>
          <div class="module-deploy-list">
            <div v-for="m in fieldModules.business" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ moduleDir('business', m.name) }}</span>
                <span v-if="!isLocalDocker && isMultiNode" class="node-target">@ {{ moduleTargetLabel('business') }}</span>
                <span v-if="fieldModuleStatus['biz-'+m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus['biz-'+m.name] }}</span>
              </div>
              <button class="btn btn-ghost btn-sm" @click="runModuleDeploy('business', m.name, true)" :disabled="busy || !fieldPaths.platformRoot">
                解压tar.zip+Load+改env+启动
              </button>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'biz'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <button class="btn btn-ghost" @click="goToStep(3)">返回</button>
            <button class="btn btn-primary" @click="completeFieldStep('business', 5)" :disabled="!fieldStepDone.middleware">
              平台业务就绪，进入 ⑥ 市政/模型
            </button>
          </div>
        </div>

        <!-- Linux step 5: 市政水厂 & 模型服务（独立包） -->
        <div v-else-if="!isLocalDocker && wizardStep === 5">
          <div class="hint-banner">
            <strong>步骤 6 / 8</strong> — 市政水厂、模型服务为<strong>独立交付包</strong>（非 platform / middleware 子目录）。
            现场惯例：改 .env IP 后 <code>docker compose up -d --build</code>，在 jar 目录构建镜像（无需 tar.zip / docker load）。
          </div>
          <p v-if="!standaloneModules.length" class="muted">
            未配置独立包目录。可在步骤 ① 填写 <code>waterwork-4.1.1</code> / <code>wpg-intelligent-model-4.1.2</code> 路径，或跳过本步。
          </p>
          <div class="module-deploy-list">
            <div v-for="m in fieldModules.standalone" :key="m.name" class="module-deploy-row">
              <div>
                <strong>{{ m.label }}</strong>
                <span class="muted"> — {{ siteForm.paths[m.pathKey] || '（未配置，可跳过）' }}</span>
                <span v-if="fieldModuleStatus['std-'+m.name]" class="badge green" style="margin-left:0.5rem">{{ fieldModuleStatus['std-'+m.name] }}</span>
              </div>
              <button
                class="btn btn-ghost btn-sm"
                @click="runStandaloneDeploy(m.pathKey, m.name)"
                :disabled="busy || !siteForm.paths[m.pathKey]"
              >
                改env+compose up --build
              </button>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length" style="margin-top:1rem">
            <div v-for="(l, i) in jobLogs" :key="'std'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions">
            <button class="btn btn-ghost" @click="goToStep(4)">返回</button>
            <button class="btn btn-primary" @click="completeFieldStep('standalone', 6)" :disabled="!fieldStepDone.business">
              {{ standaloneModules.length ? '独立包已处理，进入 ⑦ Nginx' : '跳过，进入 ⑦ Nginx' }}
            </button>
          </div>
        </div>

        <!-- Linux step 6: Nginx -->
        <div v-else-if="!isLocalDocker && wizardStep === 6">
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 7 / 8</strong> — load 镜像、解压 html、编辑下方配置文件或一键 patch IP 后 compose 启动。
          </div>
          <div class="field-grid">
            <div class="field full">
              <label>nginx 模块目录</label>
              <div class="path-row">
                <input v-model="fieldPaths.nginxDir" :placeholder="moduleDir('middleware', 'nginx') || '.../middleware/nginx'" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('nginxDir', 'dir')">浏览</button>
              </div>
              <p class="muted" style="margin:0.35rem 0 0;font-size:0.85rem">
                配置文件：<code>{{ nginxWebConfPath }}</code><br />
                前端静态：<code>{{ nginxHtmlPath }}</code>（compose 挂载为 <code>/usr/share/nginx/html</code>）
              </p>
            </div>
            <div class="field">
              <label>网关 IP（/main/ :18094）</label>
              <input v-model="fieldPaths.gatewayIP" :placeholder="primaryNodeIP || '业务节点 IP'" />
            </div>
            <div class="field">
              <label>业务 IP（:18089 / :9006，默认同网关）</label>
              <input v-model="fieldPaths.appIP" placeholder="留空=与网关相同" />
            </div>
            <div class="field">
              <label>组态 IP（/wpgEditor/ :5588）</label>
              <input v-model="fieldPaths.graphIP" placeholder="留空=与网关相同" />
            </div>
          </div>
          <div class="file-editor" v-if="fieldPaths.nginxDir">
            <div class="file-editor-head">
              <span><code>{{ nginxWebConfPath }}</code></span>
              <span>
                <button class="btn btn-ghost btn-sm" @click="loadTextFile(nginxWebConfPath)" :disabled="busy">读取</button>
                <button class="btn btn-ghost btn-sm" @click="saveTextFile(nginxWebConfPath)" :disabled="busy || !fileEditor.text">保存覆盖</button>
              </span>
            </div>
            <textarea v-model="fileEditor.text" placeholder="点「读取」加载 nginx 配置，编辑后「保存覆盖」" spellcheck="false"></textarea>
            <p v-if="fileEditor.msg" class="muted" style="padding:0.35rem 0.75rem;margin:0;font-size:0.85rem">{{ fileEditor.msg }}</p>
          </div>
          <div class="actions" :class="{ busy }">
            <button class="btn btn-primary" :class="{ 'btn-active': activeJobKey === 'nginx-patch' }" @click="runNginxPatch(true)" :disabled="busy || !fieldPaths.nginxDir">
              load 镜像 + 解压 html + 更新配置 + 启动
            </button>
            <button class="btn btn-ghost btn-sm" :class="{ 'btn-active': activeJobKey === 'nginx-patch-only' }" @click="runNginxPatch(false)" :disabled="busy || !fieldPaths.nginxDir">
              仅更新配置（不解压）
            </button>
            <button class="btn btn-ghost" @click="goToStep(5)">返回</button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="'ngx'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="fieldStepDone.nginx || nginxPatchDone">
            <button class="btn btn-primary" @click="completeFieldStep('nginx', 7)">Nginx 就绪，进入 ⑧ 防火墙</button>
          </div>
        </div>

        <!-- Linux step 7: 防火墙 -->
        <div v-else-if="!isLocalDocker && wizardStep === 7">
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 8 / 8</strong> — 先确认防火墙<strong>已开启</strong>，再批量放行 TCP 端口；firewalld 会在全部添加后执行一次 <code>--reload</code>（需 <code>root</code> 或 sudo）。
          </div>
          <p class="muted" v-if="firewallType">
            防火墙：<code>{{ firewallType }}</code>
            <span v-if="firewallRunning" class="badge green" style="margin-left:0.35rem">已开启</span>
            <span v-else class="badge" style="margin-left:0.35rem">未开启</span>
            <span v-if="firewallDetail" class="muted"> — {{ firewallDetail }}</span>
            · 共 <strong>{{ firewallPorts.length }}</strong> 个端口待放行
          </p>
          <p v-if="firewallType && !firewallRunning" class="wizard-alert muted" style="margin-top:0.75rem">
            防火墙未运行，请先执行
            <code v-if="firewallType === 'firewalld'">systemctl start firewalld && systemctl enable firewalld</code>
            <code v-else-if="firewallType === 'ufw'">ufw enable</code>
            <code v-else>启动对应防火墙服务</code>
            后再点「检查并放行」。
          </p>
          <div v-if="firewallPorts.length" class="port-chip-wrap">
            <span v-for="p in firewallPorts" :key="p" class="port-chip">{{ p }}</span>
          </div>
          <div class="actions" :class="{ busy }">
            <button class="btn btn-ghost btn-sm" @click="loadFirewallPorts" :disabled="busy">刷新状态与端口</button>
            <button
              v-if="firewallType && !firewallRunning"
              class="btn btn-ghost btn-sm"
              :class="{ 'btn-active': activeJobKey === 'firewall-start' }"
              @click="runFirewallStart"
              :disabled="busy"
            >
              {{ activeJobKey === 'firewall-start' ? '启动中…' : '启动防火墙' }}
            </button>
            <button class="btn btn-primary" :class="{ 'btn-active': activeJobKey === 'firewall-open' }" @click="runFirewallOpen" :disabled="busy || !firewallPorts.length">
              {{ activeJobKey === 'firewall-open' ? '处理中…' : '启动并放行全部端口' }}
            </button>
            <button class="btn btn-ghost" @click="goToStep(6)">返回</button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="'fw'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="firewallOpenDone">
            <button class="btn btn-primary" @click="goStatus">完成，查看服务状态</button>
          </div>
        </div>

        <!-- Windows 联调 step 1 -->
        <div v-else-if="isLocalDocker && wizardStep === 1">
          <div class="actions" style="margin-top: 0">
            <button class="btn btn-primary" @click="runPrecheck" :disabled="busy">
              {{ busy ? '体检进行中…' : '执行 Precheck' }}
            </button>
            <button class="btn btn-ghost" @click="goToStep(0)">返回</button>
          </div>
          <div v-if="precheckItems.length" class="check-list" style="margin-top: 1.25rem">
            <div v-for="(it, i) in precheckItems" :key="i" class="check-item">
              <span class="badge" :class="it.severity">{{ it.severity }}</span>
              <div>
                <strong>{{ it.name }}</strong>
                <div class="muted">{{ it.message }}</div>
                <div v-if="it.hint" class="hint-line">→ {{ it.hint }}</div>
              </div>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="precheckDone">
            <button class="btn btn-primary" @click="goToStep(2)" :disabled="precheckBlocked || !precheckDone">进入初始化</button>
          </div>
        </div>

        <!-- step 2 -->
        <div v-else-if="isLocalDocker && wizardStep === 2">
          <p class="muted">将创建目录、按需安装 Docker、放行端口。多机场景会 SSH 分发到各节点。</p>
          <div class="field-grid" style="margin-top:0.75rem">
            <div class="field full">
              <label>Docker 离线安装目录</label>
              <div class="path-row">
                <input v-model="form.dockerPackage" placeholder="含 offline_install_docker.sh 的目录" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('dockerPackage', 'dir')">浏览</button>
              </div>
            </div>
            <div class="field full">
              <label>Base 包目录（可选）</label>
              <div class="path-row">
                <input v-model="form.base" placeholder="middleware 或标准 base" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('base', 'dir')">浏览</button>
              </div>
            </div>
          </div>
          <div class="actions">
            <button class="btn btn-primary" @click="runInit" :disabled="busy">
              {{ busy ? '初始化中…' : '执行 Init' }}
            </button>
            <button class="btn btn-ghost" @click="goToStep(1)">返回</button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="initDone">
            <button class="btn btn-primary" @click="goToStep(3)">进入部署</button>
          </div>
        </div>

        <!-- step 3 -->
        <div v-else-if="isLocalDocker && wizardStep === 3">
          <div class="field-grid">
            <div class="field full">
              <label>Release 包目录</label>
              <div class="path-row">
                <input v-model="form.package" placeholder="选择含 manifest.yaml 的包目录" />
                <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('package', 'dir')">浏览</button>
              </div>
            </div>
          </div>
          <div class="actions">
            <button class="btn btn-ghost" @click="loadPreview" :disabled="busy || !form.package">预览将部署的服务</button>
            <button class="btn btn-primary" @click="runDeploy(false)" :disabled="busy || !form.package">
              {{ busy ? '部署中…' : '开始 Deploy' }}
            </button>
            <button class="btn btn-ghost" @click="runDeploy(true)" :disabled="busy || !form.package">仅渲染（Dry-run）</button>
            <button class="btn btn-ghost" @click="goToStep(2)">返回</button>
          </div>
          <div v-if="preview" class="surface" style="margin-top:1rem;padding:1rem">
            <p class="muted">版本 {{ preview.version }} · {{ preview.count }} 个服务 · profiles: {{ (preview.profiles || []).join(', ') }}</p>
            <p class="muted">端口: {{ (preview.ports || []).join(', ') }}</p>
            <table class="table" v-if="preview?.services?.length">
              <thead><tr><th>服务</th><th>层</th><th>端口</th><th>镜像</th></tr></thead>
              <tbody>
                <tr v-for="s in preview.services" :key="s.name">
                  <td>{{ s.name }}</td>
                  <td>L{{ s.layer }}</td>
                  <td>{{ s.port || '—' }}</td>
                  <td class="muted">{{ s.image }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div v-if="smokeRows.length" class="surface" style="margin-top:1rem;padding:1rem">
            <p class="muted">健康矩阵</p>
            <table class="table">
              <thead><tr><th>服务</th><th>端口</th><th>状态</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="e in smokeRows" :key="e.name">
                  <td>{{ e.name }}</td>
                  <td>{{ e.port || '—' }}</td>
                  <td><span class="badge" :class="e.ok ? 'green' : 'red'">{{ e.ok ? 'ok' : 'fail' }}</span></td>
                  <td class="muted">{{ e.message }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="deployDone" class="actions">
            <button class="btn btn-primary" @click="openStatus">查看运行状态</button>
            <button class="btn btn-ghost" @click="openReport()">打开验收报告</button>
          </div>
        </div>
      </div>
    </section>

    <!-- STATUS -->
    <section v-else-if="view === 'status'" class="panel">
      <div class="panel-head">
        <div>
          <h2>运行状态</h2>
          <p><span class="pulse"></span>一键部署用 rendered compose；现场逐步部署用 docker ps 汇总</p>
        </div>
        <button class="btn btn-ghost" @click="loadStatus">刷新</button>
      </div>
      <div class="surface">
        <p v-if="statusWarning" class="muted">{{ statusWarning }}</p>
        <table class="table" v-if="services.length">
          <thead>
            <tr>
              <th>服务</th>
              <th>状态</th>
              <th>健康</th>
              <th>镜像</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in services" :key="s.Name || s.Service">
              <td>{{ s.Service || s.Name }}</td>
              <td>{{ s.State || '—' }}</td>
              <td>{{ s.Health || s.Status || '—' }}</td>
              <td class="muted">{{ settings.privacyMode ? '******' : (s.Image || '—') }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">暂无服务数据。完成部署后在此查看。</p>
      </div>
    </section>

    <!-- LOGS -->
    <section v-else-if="view === 'logs'" class="panel">
      <div class="panel-head">
        <div>
          <h2>服务日志</h2>
          <p>WebSocket 拉取容器最近日志，适配远程桌面排障。</p>
        </div>
      </div>
      <div class="surface">
        <div class="field-grid">
          <div class="field">
            <label>服务 / 容器名</label>
            <input v-model="logService" placeholder="waterwork-center" />
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-primary" @click="startLogs" :disabled="!logService">开始监听</button>
          <button class="btn btn-ghost" @click="stopLogs">停止</button>
        </div>
        <div class="log-box" style="max-height: 480px">
          <pre style="margin:0;white-space:pre-wrap;font:inherit">{{ liveLogs || '等待日志…' }}</pre>
        </div>
      </div>
    </section>

    <!-- HISTORY / 交付单 -->
    <section v-else-if="view === 'history'" class="panel">
      <div class="panel-head">
        <div>
          <h2>交付单</h2>
          <p>本地 ~/.wpgctl/state 中的版本真相来源。</p>
        </div>
        <div class="actions" style="margin-top:0">
          <button class="btn btn-ghost" @click="loadHistory">刷新</button>
          <button class="btn btn-ghost" @click="exportDeliveries">导出全部</button>
        </div>
      </div>
      <div class="surface">
        <table class="table" v-if="deployments.length">
          <thead>
            <tr>
              <th>时间</th>
              <th>标题 / 动作</th>
              <th>站点</th>
              <th>版本</th>
              <th>操作者</th>
              <th>耗时</th>
              <th>结果</th>
              <th>验收</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in deployments.slice().reverse()" :key="d.id">
              <td>{{ formatTime(d.time) }}</td>
              <td>{{ d.title || d.action }}</td>
              <td>{{ d.siteName || d.siteCode }}</td>
              <td>{{ d.packageVersion }}</td>
              <td>{{ d.operator || '—' }}</td>
              <td>{{ d.durationSec != null ? d.durationSec + 's' : '—' }}</td>
              <td>
                <span class="badge" :class="d.success ? 'green' : 'red'">
                  {{ d.success ? 'ok' : 'fail' }}
                </span>
              </td>
              <td>
                <span class="badge" :class="d.acceptanceOk ? 'green' : 'yellow'">
                  {{ d.acceptanceOk ? 'ok' : '—' }}
                </span>
              </td>
              <td>
                <button class="btn btn-ghost btn-sm" @click="openReport(d.id)">查看报告</button>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">尚无交付记录。</p>
      </div>
    </section>

    <!-- REPORT -->
    <section v-else-if="view === 'report'" class="panel">
      <div class="panel-head">
        <div>
          <h2>验收报告</h2>
          <p>基于最近一次成功交付或指定交付单生成。</p>
        </div>
        <div class="actions" style="margin-top:0">
          <a class="btn btn-ghost" :href="reportUrl" target="_blank" rel="noopener">新窗口打开</a>
          <button class="btn btn-ghost" @click="view = 'history'">返回交付单</button>
        </div>
      </div>
      <div class="surface report-frame-wrap">
        <iframe class="report-frame" :src="reportUrl" title="验收报告"></iframe>
        <div v-if="smokeRows.length" style="margin-top:1rem">
          <p class="muted">本次会话健康矩阵</p>
          <table class="table">
            <thead><tr><th>服务</th><th>端口</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="e in smokeRows" :key="'r-' + e.name">
                <td>{{ e.name }}</td>
                <td>{{ e.port || '—' }}</td>
                <td><span class="badge" :class="e.ok ? 'green' : 'red'">{{ e.ok ? 'ok' : 'fail' }}</span></td>
                <td class="muted">{{ e.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <!-- FETCH / 包中心 -->
    <section v-else-if="view === 'fetch'" class="panel">
      <div class="panel-head">
        <div>
          <h2>包中心</h2>
          <p>本地包仓库、远程拉取与离线导入。</p>
        </div>
        <span class="badge" :class="baseReady ? 'green' : 'yellow'">
          {{ baseReady ? 'Base 就绪' : 'Base 未就绪' }}
        </span>
      </div>
      <div class="surface" style="margin-bottom:1.25rem">
        <h3 style="margin:0 0 0.75rem;font-family:var(--font-display)">扫描生成 manifest</h3>
        <p class="hint-banner">递归扫描目录内镜像 tar，自动解析服务名与版本，生成 manifest.yaml（可再人工核对）。</p>
        <div class="field-grid">
          <div class="field full">
            <label>包目录</label>
            <div class="path-row">
              <input v-model="scanForm.dir" placeholder="例如 D:/package/middleware/middleware" />
              <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('scanDir', 'dir')">浏览</button>
            </div>
          </div>
          <div class="field">
            <label>kind</label>
            <input v-model="scanForm.kind" placeholder="base 或 release" />
          </div>
          <div class="field">
            <label>包版本（可选）</label>
            <input v-model="scanForm.version" placeholder="默认 1.0.0" />
          </div>
        </div>
        <div class="actions" style="margin-top:0">
          <button class="btn btn-primary" @click="runScan(false)" :disabled="busy || !scanForm.dir">预览</button>
          <button class="btn btn-ghost" @click="runScan(true)" :disabled="busy || !scanForm.dir">生成并写入目录</button>
        </div>
        <p v-if="scanError" class="muted" style="color:var(--danger);margin-top:0.75rem">{{ scanError }}</p>
        <p v-if="scanMsg" class="muted" style="color:var(--ok);margin-top:0.75rem">{{ scanMsg }}</p>
        <div v-if="scanImages.length" class="surface" style="margin-top:1rem;padding:1rem">
          <p class="muted">识别到 {{ scanImages.length }} 个镜像</p>
          <table class="table">
            <thead><tr><th>服务</th><th>层</th><th>镜像</th><th>来源</th></tr></thead>
            <tbody>
              <tr v-for="img in scanImages" :key="img.tarPath">
                <td>{{ img.name }}</td>
                <td>L{{ img.layer }}</td>
                <td>{{ img.image }}</td>
                <td class="muted">{{ settings.privacyMode ? '******' : img.tarPath }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="scanYaml" class="field full" style="margin-top:1rem">
          <label>manifest.yaml 预览</label>
          <YamlEditor v-model="scanYaml" />
        </div>
      </div>

      <div class="surface" style="margin-bottom:1.25rem">
        <div class="panel-head" style="margin-bottom:1rem">
          <div>
            <h3 style="margin:0;font-family:var(--font-display)">本地包列表</h3>
            <p class="muted" style="margin:0.35rem 0 0">共 {{ packagesCount }} 个包</p>
          </div>
          <button class="btn btn-ghost btn-sm" @click="loadPackages">刷新</button>
        </div>
        <table class="table" v-if="packagesList.length">
          <thead>
            <tr>
              <th>名称</th>
              <th>类型</th>
              <th>版本</th>
              <th>路径</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in packagesList" :key="p.path">
              <td>{{ p.name }}</td>
              <td>{{ p.kind || '—' }}</td>
              <td>{{ p.version || '—' }}</td>
              <td class="muted">{{ settings.privacyMode ? '******' : p.path }}</td>
              <td>
                <button class="btn btn-ghost btn-sm" @click="usePackagePath(p.path)">用于部署</button>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">本地尚无带 manifest.yaml 的包。</p>
      </div>
      <div class="surface">
        <h3 style="margin:0 0 1rem;font-family:var(--font-display)">拉包 / 本地导入</h3>
        <div class="field-grid">
          <div class="field">
            <label>包名（远程拉取）</label>
            <input v-model="fetchForm.name" placeholder="release-4.0.2 或 patch-4.0.3" />
          </div>
          <div class="field">
            <label>拉包基址 URL（可选）</label>
            <input v-model="fetchForm.from" placeholder="https://packages.example.com/packages" />
          </div>
          <div class="field full">
            <label>或：本地分卷 / 完整包目录</label>
            <div class="path-row">
              <input v-model="fetchForm.local" placeholder="包目录，或含 .zip / .tar.gz 的目录" />
              <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('fetchLocal', 'dir')">浏览</button>
            </div>
            <p class="hint-banner" style="margin-top:0.5rem;margin-bottom:0">
              目录内若是 <code>.zip</code> / <code>.tar.zip</code>，在浏览窗口点「解压 ZIP / 展开 TAR」；本地导入也会自动尝试解压后再合并 tar.gz。
            </p>
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-primary" @click="runFetch" :disabled="busy || (!fetchForm.name && !fetchForm.local)">
            {{ busy ? '进行中…' : '开始拉包 / 导入' }}
          </button>
        </div>
        <div class="log-box" v-if="jobLogs.length">
          <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
        </div>
        <p v-if="fetchResultDir" class="muted" style="color:var(--ok);margin-top:1rem">
          包已就绪：{{ settings.privacyMode ? '******' : fetchResultDir }}
          <button class="btn btn-ghost btn-sm" style="margin-left:0.5rem" @click="useFetchedPackage">用于部署向导</button>
        </p>
      </div>
    </section>

    <!-- UPGRADE / ROLLBACK -->
    <section v-else-if="view === 'upgrade'" class="panel">
      <div class="panel-head">
        <div>
          <h2>升级 / 回滚</h2>
          <p>日常 1~3 服务补丁升级；失败自动回滚镜像（SQL 不自动回滚）。</p>
        </div>
      </div>
      <div class="surface">
        <p v-if="latestVersion" class="hint-banner">当前成功版本：<strong>{{ latestVersion }}</strong></p>
        <p v-else class="hint-banner">尚无成功部署记录，请先完成首次 deploy。</p>

        <h3 style="margin:1.25rem 0 0.75rem;font-family:var(--font-display)">补丁升级</h3>
        <div class="field-grid">
          <div class="field full">
            <label>Patch 包目录</label>
            <div class="path-row">
              <input v-model="upgradeForm.patchDir" placeholder="选择含 manifest.yaml 的 patch 目录" />
              <button type="button" class="btn btn-ghost btn-sm" @click="openPicker('patchDir', 'dir')">浏览</button>
            </div>
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-primary" @click="runUpgrade" :disabled="busy || !upgradeForm.patchDir">
            {{ busy ? '升级中…' : '执行升级' }}
          </button>
        </div>

        <hr class="sep" />

        <h3 style="margin:0 0 0.75rem;font-family:var(--font-display)">回滚</h3>
        <div class="field-grid">
          <div class="field">
            <label>目标版本（空=上一成功版本）</label>
            <input v-model="upgradeForm.rollbackTo" placeholder="例如 4.0.2" />
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-ghost" @click="runRollback" :disabled="busy">执行回滚</button>
        </div>

        <div class="log-box" v-if="jobLogs.length">
          <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
        </div>
      </div>
    </section>

    <!-- 路径选择器 -->
    <div v-if="picker.open" class="modal-mask" @click.self="picker.open = false">
      <div class="modal">
        <div class="modal-head">
          <div>
            <strong>{{ picker.mode === 'yaml' ? '选择 YAML 文件' : '选择目录' }}</strong>
            <div class="muted" style="font-size:0.85rem;margin-top:0.25rem">{{ picker.current || '选择盘符 / 根目录' }}</div>
          </div>
          <button class="btn btn-ghost btn-sm" @click="picker.open = false">关闭</button>
        </div>
        <div class="modal-toolbar">
          <button class="btn btn-ghost btn-sm" @click="browseFS(picker.parent)" :disabled="!picker.parent && picker.current">上级</button>
          <button class="btn btn-ghost btn-sm" @click="browseFS('')">根 / 盘符</button>
          <button
            v-if="picker.mode === 'dir' && picker.current"
            class="btn btn-ghost btn-sm"
            @click="expandPickerDir"
            :disabled="picker.expanding"
          >{{ picker.expanding ? '解压中…' : '解压 ZIP / 展开 TAR' }}</button>
          <button
            v-if="picker.mode === 'dir' && picker.current"
            class="btn btn-primary btn-sm"
            @click="confirmPicker(picker.current)"
          >选择当前目录</button>
        </div>
        <p v-if="picker.mode === 'dir'" class="muted" style="padding:0 1rem;font-size:0.85rem">
          目录内若有 <code>.zip</code> / <code>.tar.zip</code>，可先点「解压 ZIP / 展开 TAR」；含 <code>[manifest]</code> 用于部署，含 <code>[docker]</code> 用于 Docker 离线安装。
        </p>
        <div class="fs-list">
          <button
            v-for="e in picker.entries"
            :key="e.path"
            type="button"
            class="fs-item"
            @click="onFsClick(e)"
            @dblclick="onFsDblClick(e)"
          >
            <span class="fs-icon">{{ fsEntryIcon(e) }}</span>
            <span>{{ e.name }}</span>
          </button>
          <p v-if="!(picker.entries && picker.entries.length)" class="muted" style="padding:1rem">空目录或无可选文件</p>
        </div>
        <p v-if="picker.expandMsg" class="muted" style="color:var(--ok);padding:0 1rem 0.5rem">{{ picker.expandMsg }}</p>
        <p v-if="picker.error" class="muted" style="color:var(--danger);padding:0 1rem 1rem">{{ picker.error }}</p>
      </div>
    </div>

    <!-- 确认弹窗 -->
    <div v-if="confirmModal.open" class="modal-mask" @click.self="answerConfirm(false)">
      <div class="modal confirm-modal">
        <div class="modal-head">
          <div>
            <strong>{{ confirmModal.title }}</strong>
          </div>
          <button class="btn btn-ghost btn-sm" @click="answerConfirm(false)">关闭</button>
        </div>
        <div class="confirm-body">
          <p>{{ confirmModal.text }}</p>
        </div>
        <div class="modal-toolbar" style="border-bottom:0;border-top:1px solid var(--line)">
          <button class="btn btn-ghost btn-sm" @click="answerConfirm(false)">取消</button>
          <button class="btn btn-primary btn-sm" @click="answerConfirm(true)">确认继续</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import YamlEditor from './YamlEditor.vue'

const view = ref('home')
const wizardStep = ref(0)
const maxReachedStep = ref(0)
const site = ref(null)
const siteYaml = ref('')
const sitePath = ref('')
const siteError = ref('')
const siteSaveMsg = ref('')
const siteEditMode = ref('form')

/** 交付默认账号（与公司标准 .env 一致，现场非必要不改） */
const NODE_ROLE_OPTIONS = [
  { id: 'database', label: 'database' },
  { id: 'middleware', label: 'middleware' },
  { id: 'platform', label: 'platform' },
  { id: 'waterwork', label: 'waterwork' },
  { id: 'monitor', label: 'monitor' },
]

const ALL_SINGLE_ROLES = ['database', 'middleware', 'platform', 'waterwork', 'monitor']

function defaultNode(name, ip, roles) {
  return { name, ip, sshUser: 'root', sshPort: 22, roles: roles.slice() }
}

const DEFAULT_CREDS = {
  nacosUser: 'nacos',
  nacosPassword: 'wpg@nice#LKsalk98',
  nacosNamespace: 'intergrate',
  mysqlUser: 'wpg',
  mysqlPassword: 'DmJme(ZFl9txW@2P',
  pgsqlUser: 'wpg',
  pgsqlPassword: 't7u!m0Wpyu7EfbVN',
  pgsqlPort: 5433,
  redisPassword: 'SJ(Qu%(kfXQBjxyT',
}

const deployTopology = ref('single')
const sshCreds = reactive({ password: '', keyPath: '' })

const siteForm = reactive({
  site: { name: '', code: '' },
  nodes: [defaultNode('app-node', '127.0.0.1', ALL_SINGLE_ROLES)],
  middleware: {
    nacos: {
      host: '127.0.0.1',
      port: 8848,
      namespace: DEFAULT_CREDS.nacosNamespace,
      username: DEFAULT_CREDS.nacosUser,
      password: DEFAULT_CREDS.nacosPassword,
    },
    mysql: {
      disabled: false,
      host: '127.0.0.1',
      port: 3306,
      user: DEFAULT_CREDS.mysqlUser,
      password: DEFAULT_CREDS.mysqlPassword,
    },
    pgsql: {
      host: '127.0.0.1',
      port: DEFAULT_CREDS.pgsqlPort,
      user: DEFAULT_CREDS.pgsqlUser,
      password: DEFAULT_CREDS.pgsqlPassword,
    },
    redis: {
      host: '127.0.0.1',
      port: 6377,
      password: DEFAULT_CREDS.redisPassword,
    },
    kafka: { host: '127.0.0.1', port: 9092 },
  },
  paths: { workspace: '', nginxHtml: '', waterwork: '', intelligentModel: '' },
})

const moduleDefs = [
  { id: 'platform', label: '平台基础' },
  { id: 'device', label: '设备管理' },
  { id: 'alarm', label: '报警' },
  { id: 'gis', label: 'GIS' },
  { id: 'monitor', label: '监控' },
  { id: 'graph', label: '组态' },
]
const selectedModules = reactive({
  platform: true,
  device: false,
  alarm: false,
  gis: false,
  monitor: true,
  graph: false,
})

const passwordVisible = reactive({
  nacos: false,
  mysql: false,
  redis: false,
  pgsql: false,
})

const localStepDefs = [
  { key: 'site', idx: '01', title: '节点配置', purpose: '填写本机联调节点信息、中间件 IP 与业务模块，保存到 site.yaml。' },
  { key: 'precheck', idx: '02', title: '环境体检', purpose: '检查 Docker、端口、磁盘等，红色项会阻断后续部署。' },
  { key: 'init', idx: '03', title: '初始化', purpose: '创建目录、安装 Docker（如需要）、放行端口。' },
  { key: 'deploy', idx: '04', title: '部署启动', purpose: '按 manifest 拉包并启动全部服务。' },
]

/** Linux 现场 8 步 SOP（Tab 可回看，前进软锁定） */
const fieldStepDefs = [
  { key: 'site', idx: '01', title: '节点配置', purpose: '单机或多机：填写机器清单（IP、SSH、角色），中间件 Host 按角色自动映射；保存后写入 site.yaml。' },
  { key: 'docker', idx: '02', title: '安装 Docker', purpose: '离线安装 Docker；docker.service 的 --graph 指向挂载盘 docker_data 目录（与 workspace 同盘）。安装成功会输出 docker -v。' },
  { key: 'database', idx: '03', title: '数据库', purpose: 'MySQL / PostgreSQL / PostGIS / MongoDB：有 zip 则解压，已有 .tar 则直接 load，再 compose up。' },
  { key: 'middleware', idx: '04', title: '中间件', purpose: 'Redis → Kafka → Nacos → … 逐个解压/load/启动；Nacos 启动后自动导入配置（需先填配置目录）；Kafka 会改 compose 内 KAFKA_ADVERTISED_LISTENERS。' },
  { key: 'business', idx: '05', title: '平台业务', purpose: 'platform 各服务：展开 tar.zip → load → 批量改 .env IP → compose up。' },
  { key: 'standalone', idx: '06', title: '市政/模型', purpose: '独立交付包：改 .env 后 compose up --build（无需 tar load）。' },
  { key: 'nginx', idx: '07', title: 'Nginx', purpose: 'load 镜像、解压前端、编辑 http-web-8877.conf 并启动反向代理。' },
  { key: 'firewall', idx: '08', title: '防火墙', purpose: '启动 firewalld/ufw 并批量放行 manifest 端口（需 root 或 sudo）。' },
]
const fieldStepDone = reactive({
  site: false,
  docker: false,
  database: false,
  middleware: false,
  business: false,
  standalone: false,
  nginx: false,
  firewall: false,
})
const firewallPorts = ref([])
const firewallType = ref('')
const firewallRunning = ref(false)
const firewallDetail = ref('')
const firewallOpenDone = ref(false)
const fieldPaths = reactive({
  middlewareRoot: '',
  platformRoot: '',
  nginxDir: '',
  nacosConfigDir: '',
  gatewayIP: '',
  appIP: '',
  graphIP: '',
})
const fieldModuleStatus = reactive({})
const nginxPatchDone = ref(false)
const nacosImportDone = ref(false)
const fieldModules = {
  database: [
    { name: 'mysql', label: 'MySQL', skippable: true },
    { name: 'pgsql', label: 'PostgreSQL' },
    { name: 'postgis', label: 'PostGIS' },
    { name: 'mongodb', label: 'MongoDB' },
  ],
  middleware: [
    { name: 'redis', label: 'Redis' },
    { name: 'kafka', label: 'Kafka' },
    { name: 'nacos', label: 'Nacos' },
    { name: 'minio', label: 'MinIO' },
    { name: 'influxdb', label: 'InfluxDB' },
    { name: 'emqx', label: 'EMQX' },
  ],
  business: [
    { name: 'public', label: '平台 public' },
    { name: 'device', label: '设备 device' },
    { name: 'alarm', label: '报警 alarm' },
    { name: 'graph', label: '组态 graph' },
    { name: 'gis', label: 'GIS' },
    { name: 'monitor', label: '监控 monitor' },
    { name: 'report-center', label: '报表' },
    { name: 'out-work', label: 'out-work' },
  ],
  standalone: [
    { name: 'waterwork', label: '市政水厂', pathKey: 'waterwork', hint: 'waterwork-4.1.1' },
    { name: 'intelligent-model', label: '模型服务', pathKey: 'intelligentModel', hint: 'wpg-intelligent-model-4.1.2' },
  ],
}

const standaloneModules = computed(() =>
  fieldModules.standalone.filter((m) => (siteForm.paths[m.pathKey] || '').trim()),
)

const fieldDatabaseModules = computed(() => {
  if (siteForm.middleware.mysql.disabled) {
    return fieldModules.database.filter((m) => m.name !== 'mysql')
  }
  return fieldModules.database
})

const busy = ref(false)
const activeJobKey = ref('')
const fileEditor = reactive({ path: '', text: '', loading: false, msg: '' })
const jobLogs = ref([])
const precheckItems = ref([])
const precheckDone = ref(false)
const precheckBlocked = ref(false)
const initDone = ref(false)
const deployDone = ref(false)
const preview = ref(null)
const smokeRows = ref([])
const services = ref([])
const statusWarning = ref('')
const deployments = ref([])
const logService = ref('')
const liveLogs = ref('')
let logWs = null
let jobWs = null

const form = reactive({
  manifest: 'configs/examples/manifest.release.example.yaml',
  package: '',
  base: '',
  dockerPackage: '',
})

const fetchForm = reactive({
  name: '',
  from: '',
  local: '',
})
const fetchResultDir = ref('')

const scanForm = reactive({
  dir: '',
  kind: 'base',
  version: '',
})
const scanYaml = ref('')
const scanImages = ref([])
const scanError = ref('')
const scanMsg = ref('')

const hostCandidates = ref([])
const selectedHost = ref('')
const hostFillMsg = ref('')
const hostOptions = computed(() => (Array.isArray(hostCandidates.value) ? hostCandidates.value : []))

const upgradeForm = reactive({
  patchDir: '',
  rollbackTo: '',
})
const latestVersion = ref('')
const packagesList = ref([])
const baseReady = ref(false)
const packagesCount = ref(0)
const reportId = ref('')

const settings = reactive({
  operator: '',
  privacyMode: false,
  scenario: 'linux',
})

const confirmModal = reactive({
  open: false,
  title: '',
  text: '',
  resolve: null,
})

const picker = reactive({
  open: false,
  target: '',
  mode: 'dir',
  current: '',
  parent: '',
  entries: [],
  error: '',
  expandMsg: '',
  expanding: false,
})

const runtimeOS = ref('')
const runtimeArch = ref('')
const dockerOk = ref(null)
const dockerMsg = ref('')
const deployHint = ref('')

const siteName = computed(() => {
  const p = site.value
  if (!p) return '未加载站点'
  return p.site?.name || p.Site?.Name || p.Site?.name || '未加载站点'
})
const siteCode = computed(() => {
  const p = site.value
  if (!p) return '—'
  return p.site?.code || p.Site?.Code || p.Site?.code || '—'
})
const siteLoaded = computed(() => !!site.value)
const reportUrl = computed(() => (reportId.value ? `/api/report?id=${encodeURIComponent(reportId.value)}` : '/api/report'))
const isLocalDocker = computed(() => settings.scenario === 'windows')

const nginxHtmlPath = computed(() => {
  const base = fieldPaths.nginxDir || (fieldPaths.middlewareRoot ? joinPath(fieldPaths.middlewareRoot, 'nginx') : '')
  if (!base) return ''
  return joinPath(base, 'html')
})

const nginxWebConfPath = computed(() => {
  const base = fieldPaths.nginxDir || (fieldPaths.middlewareRoot ? joinPath(fieldPaths.middlewareRoot, 'nginx') : '')
  if (!base) return 'middleware/nginx/conf/conf.d/http-web-8877.conf'
  return joinPath(joinPath(base, 'conf'), 'conf.d/http-web-8877.conf').replace(/\\/g, '/')
})
const heroLead = computed(() =>
  isLocalDocker.value
    ? '本机 Docker 联调模式：用 Desktop 跑通交付流程，适合演示与研发验证。'
    : 'Linux 现场交付模式：从打包到验收，一条可审计、可回滚的标准作业路径。',
)
const modeHint = computed(() =>
  isLocalDocker.value
    ? '已开启「本机 Docker」。需要回到现场交付时，关闭顶栏或首页开关即可。'
    : '默认现场 Linux 交付。需要在本机用 Docker Desktop 联调时，打开「本机 Docker」开关。',
)
const platformHint = computed(() => {
  if (isLocalDocker.value) {
    return runtimeOS.value === 'windows'
      ? '当前机器是 Windows，适合 Docker Desktop 本机联调。'
      : '本机 Docker 联调已开启；请确保本机 Docker 可用。'
  }
  if (runtimeOS.value === 'linux') {
    return '当前是 Linux，可直接作为现场主控机执行完整交付。'
  }
  if (runtimeOS.value === 'windows') {
    return '默认按现场 Linux 交付引导。若要在本机联调，请打开「本机 Docker」。'
  }
  return '默认 Linux 现场交付；可用开关切换为本机 Docker 联调。'
})
const envChipText = computed(() => {
  const arch = runtimeArch.value || 'amd64'
  const modeTag = isLocalDocker.value ? '本机' : '现场'
  if (runtimeOS.value === 'windows') {
    if (dockerOk.value === true) return `${modeTag} · Windows · ${arch} · Desktop 就绪`
    if (dockerOk.value === false) return `${modeTag} · Windows · ${arch} · Desktop 未就绪`
    return `${modeTag} · Windows · ${arch}`
  }
  if (runtimeOS.value) {
    if (dockerOk.value === true) return `${modeTag} · ${runtimeOS.value} · ${arch} · Docker OK`
    if (dockerOk.value === false) return `${modeTag} · ${runtimeOS.value} · ${arch} · Docker 异常`
    return `${modeTag} · ${runtimeOS.value} · ${arch}`
  }
  return modeTag
})

const wizardStepDefs = computed(() => (isLocalDocker.value ? localStepDefs : fieldStepDefs))

const currentStepPurpose = computed(() => wizardStepDefs.value[wizardStep.value]?.purpose || '')

const primaryNodeIP = computed(() => siteForm.nodes[0]?.ip || '127.0.0.1')

const isMultiNode = computed(
  () => !isLocalDocker.value && deployTopology.value === 'multi' && siteForm.nodes.length > 1,
)

function isStepDone(key) {
  if (key === 'site') return fieldStepDone.site
  if (isLocalDocker.value) {
    if (key === 'precheck') return precheckDone.value && !precheckBlocked.value
    if (key === 'init') return initDone.value
    if (key === 'deploy') return deployDone.value
    return false
  }
  return !!fieldStepDone[key]
}

function canGoToStep(i) {
  if (busy.value) return i === wizardStep.value
  if (i === wizardStep.value) return true
  if (i === 0) return true
  if (i <= maxReachedStep.value) return true
  if (i === wizardStep.value + 1) {
    const prev = wizardStepDefs.value[i - 1]?.key
    return prev ? isStepDone(prev) : false
  }
  return false
}

function stepTabClass(i) {
  const key = wizardStepDefs.value[i]?.key
  return {
    active: wizardStep.value === i,
    done: isStepDone(key) || wizardStep.value > i,
    locked: !canGoToStep(i),
  }
}

async function prepareSiteStep({ requireDocker = false } = {}) {
  if (siteEditMode.value === 'form') {
    await saveSiteFormOnly()
  } else {
    await api('/api/site', {
      method: 'PUT',
      body: JSON.stringify({ yaml: siteYaml.value }),
    })
  }
  if (!fieldPaths.middlewareRoot) {
    siteError.value = '请填写 middleware 根目录'
    return false
  }
  if (requireDocker && !form.dockerPackage) {
    siteError.value = '请填写 Docker 离线安装目录'
    return false
  }
  fieldStepDone.site = true
  fieldPaths.gatewayIP = fieldPaths.gatewayIP || primaryNodeIP.value
  if (!fieldPaths.nginxDir && fieldPaths.middlewareRoot) {
    fieldPaths.nginxDir = joinPath(fieldPaths.middlewareRoot, 'nginx')
  }
  return true
}

/** 向导底部按钮等可信跳转：不做 Tab 锁定校验 */
function applyWizardStep(i) {
  siteError.value = ''
  wizardStep.value = i
  maxReachedStep.value = Math.max(maxReachedStep.value, i)
  if (i === 7 && !isLocalDocker.value) loadFirewallPorts()
}

async function goToStep(i, opts = {}) {
  if (i === wizardStep.value) return
  if (opts.internal === true) {
    applyWizardStep(i)
    return
  }

  // 从站点配置 Tab 点前进：等同保存并解锁（不必先点底部按钮）
  if (
    i > wizardStep.value &&
    wizardStep.value === 0 &&
    !fieldStepDone.site &&
    !isLocalDocker.value
  ) {
    busy.value = true
    try {
      const ok = await prepareSiteStep()
      if (!ok) return
    } catch (e) {
      siteError.value = e.message
      return
    } finally {
      busy.value = false
    }
  }

  if (!canGoToStep(i)) {
    if (i > wizardStep.value) {
      if (busy.value) {
        siteError.value = '任务进行中，请稍后再切换步骤'
      } else {
        const prev = wizardStepDefs.value[i - 1]
        if (wizardStep.value === 0 && !fieldStepDone.site && isLocalDocker.value) {
          siteError.value = '请先点击下方「下一步：环境体检」保存站点配置'
        } else if (prev) {
          siteError.value = `请先完成「${prev.title}」后再进入（或点击该步底部「就绪 / 下一步」）`
        } else {
          siteError.value = '请先完成上一步后再进入'
        }
      }
    }
    return
  }
  applyWizardStep(i)
}

function logClass(line) {
  if (String(line).includes('ERROR')) return 'err'
  if (String(line).includes('完成') || String(line).includes('ok')) return 'ok'
  return ''
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok && res.status !== 202) {
    throw new Error(data.error || res.statusText)
  }
  return data
}

async function loadSettings() {
  try {
    const st = await api('/api/settings')
    settings.operator = st.operator || ''
    settings.privacyMode = !!st.privacyMode
    settings.scenario = st.scenario === 'windows' ? 'windows' : 'linux'
  } catch {
    /* keep defaults */
  }
}

async function saveSettings() {
  try {
    await api('/api/settings', {
      method: 'PUT',
      body: JSON.stringify({
        mode: 'implementer',
        operator: settings.operator,
        privacyMode: settings.privacyMode,
        scenario: settings.scenario,
      }),
    })
  } catch (e) {
    console.warn('save settings failed', e)
  }
}

async function setScenario(scenario) {
  settings.scenario = scenario === 'windows' ? 'windows' : 'linux'
  await saveSettings()
}

async function onLocalDockerToggle(ev) {
  await setScenario(ev.target.checked ? 'windows' : 'linux')
}

async function loadPackages() {
  try {
    const data = await api('/api/packages')
    packagesList.value = data.packages || []
    baseReady.value = !!data.baseReady
    packagesCount.value = data.count ?? packagesList.value.length
  } catch {
    packagesList.value = []
    baseReady.value = false
    packagesCount.value = 0
  }
}

async function loadLatest() {
  try {
    const data = await api('/api/latest')
    latestVersion.value = data.latest?.packageVersion || ''
  } catch {
    latestVersion.value = ''
  }
}

function askConfirm(title, text) {
  return new Promise((resolve) => {
    confirmModal.open = true
    confirmModal.title = title
    confirmModal.text = text
    confirmModal.resolve = resolve
  })
}

function answerConfirm(ok) {
  confirmModal.open = false
  const resolve = confirmModal.resolve
  confirmModal.resolve = null
  if (resolve) resolve(ok)
}

async function loadSite() {
  siteError.value = ''
  siteSaveMsg.value = ''
  try {
    const data = await api('/api/site')
    siteYaml.value = data.yaml || ''
    sitePath.value = data.path || ''
    site.value = data.parsed
    if (data.parsed) {
      fillSiteForm(data.parsed)
      if (data.exists) {
        fieldStepDone.site = true
        maxReachedStep.value = Math.max(maxReachedStep.value, 1)
      }
    } else {
      applyDefaultCreds(true)
    }
    if (data.error) {
      siteError.value = data.error
    }
    if (data.message) {
      siteSaveMsg.value = data.message
    }
  } catch (e) {
    site.value = null
    siteError.value = e.message
    applyDefaultCreds(true)
  }
}

function fillSiteForm(parsed) {
  if (!parsed) return
  // API JSON 可能是小写 yaml 风格，也可能是 Go 默认导出的 PascalCase
  const siteMeta = parsed.site || parsed.Site || {}
  const profiles = parsed.profiles || parsed.Profiles || []
  const nodes = parsed.nodes || parsed.Nodes || []
  const m = parsed.middleware || parsed.Middleware || {}
  const paths = parsed.paths || parsed.Paths || {}
  const nacos = m.nacos || m.Nacos || {}
  const mysql = m.mysql || m.MySQL || m.Mysql || {}
  const redis = m.redis || m.Redis || {}
  const kafka = m.kafka || m.Kafka || {}
  const pgsql = m.pgsql || m.PgSQL || m.Pgsql || {}

  siteForm.site.name = siteMeta.name || siteMeta.Name || ''
  siteForm.site.code = siteMeta.code || siteMeta.Code || ''
  for (const mod of moduleDefs) {
    selectedModules[mod.id] = profiles.includes(mod.id)
  }
  if (!profiles.length) {
    selectedModules.platform = true
    selectedModules.monitor = true
  }
  if (nodes.length) {
    siteForm.nodes = nodes.map((n, i) => {
      const ssh = n.ssh || n.SSH || {}
      const roles = n.roles || n.Roles || []
      const row = defaultNode(
        n.name || n.Name || `node-${i + 1}`,
        n.ip || n.IP || '127.0.0.1',
        roles.length ? roles : ALL_SINGLE_ROLES,
      )
      row.sshUser = ssh.user || ssh.User || 'root'
      row.sshPort = ssh.port || ssh.Port || 22
      return row
    })
    deployTopology.value = nodes.length > 1 ? 'multi' : 'single'
  } else {
    siteForm.nodes = [defaultNode('app-node', '127.0.0.1', ALL_SINGLE_ROLES)]
    deployTopology.value = 'single'
  }
  siteForm.middleware.nacos.host = nacos.host || nacos.Host || siteForm.middleware.nacos.host
  siteForm.middleware.nacos.port = nacos.port || nacos.Port || 8848
  siteForm.middleware.nacos.namespace =
    nacos.namespace || nacos.Namespace || siteForm.site.code || DEFAULT_CREDS.nacosNamespace
  siteForm.middleware.nacos.username = nacos.username || nacos.Username || DEFAULT_CREDS.nacosUser
  siteForm.middleware.nacos.password = resolveLoadedPassword(
    nacos.password || nacos.Password,
    DEFAULT_CREDS.nacosPassword,
  )
  siteForm.middleware.mysql.disabled = !!(mysql.disabled || mysql.Disabled)
  siteForm.middleware.mysql.host = mysql.host || mysql.Host || siteForm.middleware.mysql.host
  siteForm.middleware.mysql.port = mysql.port || mysql.Port || 3306
  siteForm.middleware.mysql.user = mysql.user || mysql.User || DEFAULT_CREDS.mysqlUser
  siteForm.middleware.mysql.password = resolveLoadedPassword(
    mysql.password || mysql.Password,
    DEFAULT_CREDS.mysqlPassword,
  )
  siteForm.middleware.pgsql.host =
    pgsql.host ||
    pgsql.Host ||
    siteForm.middleware.pgsql.host ||
    siteForm.middleware.mysql.host
  siteForm.middleware.pgsql.port = pgsql.port || pgsql.Port || DEFAULT_CREDS.pgsqlPort
  siteForm.middleware.pgsql.user = pgsql.user || pgsql.User || DEFAULT_CREDS.pgsqlUser
  siteForm.middleware.pgsql.password = resolveLoadedPassword(
    pgsql.password || pgsql.Password,
    DEFAULT_CREDS.pgsqlPassword,
  )
  siteForm.middleware.redis.host = redis.host || redis.Host || siteForm.middleware.redis.host
  siteForm.middleware.redis.port = redis.port || redis.Port || 6377
  siteForm.middleware.redis.password = resolveLoadedPassword(
    redis.password || redis.Password,
    DEFAULT_CREDS.redisPassword,
  )
  siteForm.middleware.kafka.host = kafka.host || kafka.Host || siteForm.middleware.kafka.host
  siteForm.middleware.kafka.port = kafka.port || kafka.Port || 9092
  siteForm.paths.workspace = paths.workspace || paths.Workspace || ''
  siteForm.paths.nginxHtml = paths.nginxHtml || paths.NginxHTML || paths.NginxHtml || ''
  siteForm.paths.waterwork = paths.waterwork || paths.Waterwork || ''
  siteForm.paths.intelligentModel = paths.intelligentModel || paths.IntelligentModel || ''
}

/** 脱敏回显保持空串（保存时沿用文件原值）；缺省则填交付默认密码 */
function resolveLoadedPassword(raw, fallback) {
  if (raw === '******') return ''
  if (raw) return raw
  return fallback
}

function applyDefaultCreds(force = false) {
  const put = (obj, key, val) => {
    if (force || !obj[key]) obj[key] = val
  }
  put(siteForm.middleware.nacos, 'username', DEFAULT_CREDS.nacosUser)
  put(siteForm.middleware.nacos, 'namespace', DEFAULT_CREDS.nacosNamespace)
  put(siteForm.middleware.nacos, 'password', DEFAULT_CREDS.nacosPassword)
  if (!siteForm.middleware.mysql.disabled) {
    put(siteForm.middleware.mysql, 'user', DEFAULT_CREDS.mysqlUser)
    put(siteForm.middleware.mysql, 'password', DEFAULT_CREDS.mysqlPassword)
  }
  put(siteForm.middleware.pgsql, 'user', DEFAULT_CREDS.pgsqlUser)
  put(siteForm.middleware.pgsql, 'password', DEFAULT_CREDS.pgsqlPassword)
  if (force || !siteForm.middleware.pgsql.port) {
    siteForm.middleware.pgsql.port = DEFAULT_CREDS.pgsqlPort
  }
  put(siteForm.middleware.redis, 'password', DEFAULT_CREDS.redisPassword)
  hostFillMsg.value = force
    ? '已填充公司标准账号密码'
    : '已补全空缺的默认账号密码'
}

function switchSiteMode(mode) {
  siteEditMode.value = mode
}

function currentProfiles() {
  const out = moduleDefs.map((m) => m.id).filter((id) => selectedModules[id])
  if ((siteForm.paths.waterwork || '').trim()) {
    out.push('waterwork')
  }
  if ((siteForm.paths.intelligentModel || '').trim()) {
    out.push('intelligent-model')
  }
  return [...new Set(out)]
}

function buildFormNodes() {
  if (deployTopology.value === 'single' || isLocalDocker.value) {
    const n = siteForm.nodes[0] || defaultNode('app-node', primaryNodeIP.value, ALL_SINGLE_ROLES)
    return [
      {
        name: n.name || 'app-node',
        ip: n.ip || primaryNodeIP.value,
        ssh: { user: n.sshUser || 'root', port: Number(n.sshPort) || 22 },
        roles: ALL_SINGLE_ROLES,
      },
    ]
  }
  return siteForm.nodes.map((n, i) => ({
    name: (n.name || '').trim() || `node-${i + 1}`,
    ip: n.ip,
    ssh: { user: n.sshUser || 'root', port: Number(n.sshPort) || 22 },
    roles: (n.roles && n.roles.length ? n.roles : ['middleware']).slice(),
  }))
}

function nodeIPForRole(role) {
  const n = siteForm.nodes.find((x) => x.roles && x.roles.includes(role))
  return n?.ip || primaryNodeIP.value
}

function applyMiddlewareFromRoles() {
  const dbIP = nodeIPForRole('database')
  const mwIP = nodeIPForRole('middleware')
  if (dbIP) {
    if (!siteForm.middleware.mysql.disabled) siteForm.middleware.mysql.host = dbIP
    siteForm.middleware.pgsql.host = dbIP
    siteForm.middleware.redis.host = dbIP
  }
  if (mwIP) {
    siteForm.middleware.nacos.host = mwIP
    siteForm.middleware.kafka.host = mwIP
  }
  hostFillMsg.value = `已按角色映射：database→${dbIP}，middleware→${mwIP}`
}

function setDeployTopology(mode) {
  deployTopology.value = mode
  if (mode === 'single' && siteForm.nodes.length > 1) {
    const first = siteForm.nodes[0]
    siteForm.nodes = [defaultNode(first.name || 'app-node', first.ip, ALL_SINGLE_ROLES)]
  }
  if (mode === 'multi' && siteForm.nodes.length === 1) {
    applyDualNodeTemplate()
  }
}

function applyDualNodeTemplate() {
  deployTopology.value = 'multi'
  const masterIP = siteForm.nodes[0]?.ip || primaryNodeIP.value
  siteForm.nodes = [
    defaultNode('db-node', masterIP, ['database']),
    defaultNode('app-node', masterIP, ['middleware', 'platform', 'waterwork', 'monitor']),
  ]
  applyMiddlewareFromRoles()
  hostFillMsg.value = '已套用 db-node + app-node 双机模板，请修改各机 IP'
}

function addNode() {
  siteForm.nodes.push(defaultNode(`node-${siteForm.nodes.length + 1}`, '', ['middleware']))
}

function removeNode(idx) {
  if (siteForm.nodes.length <= 1) return
  siteForm.nodes.splice(idx, 1)
}

function toggleNodeRole(node, roleId) {
  const i = node.roles.indexOf(roleId)
  if (i >= 0) node.roles.splice(i, 1)
  else node.roles.push(roleId)
}

function moduleTargetLabel(phase) {
  const role = { database: 'database', middleware: 'middleware', business: 'platform' }[phase]
  const n = siteForm.nodes.find((x) => x.roles && x.roles.includes(role))
  if (!n) return '未分配角色'
  return `${n.name} (${n.ip})`
}

function buildFormPayload() {
  const profiles = currentProfiles()
  const ns = siteForm.middleware.nacos.namespace || siteForm.site.code
  return {
    site: { name: siteForm.site.name, code: siteForm.site.code },
    nodes: buildFormNodes(),
    profiles,
    middleware: {
      nacos: {
        host: siteForm.middleware.nacos.host,
        port: Number(siteForm.middleware.nacos.port) || 8848,
        namespace: ns,
        username: siteForm.middleware.nacos.username || DEFAULT_CREDS.nacosUser,
        password: siteForm.middleware.nacos.password,
      },
      mysql: siteForm.middleware.mysql.disabled
        ? { disabled: true }
        : {
            host: siteForm.middleware.mysql.host,
            port: Number(siteForm.middleware.mysql.port) || 3306,
            user: siteForm.middleware.mysql.user || DEFAULT_CREDS.mysqlUser,
            password: siteForm.middleware.mysql.password,
          },
      pgsql: {
        host:
          siteForm.middleware.pgsql.host ||
          (siteForm.middleware.mysql.disabled
            ? primaryNodeIP.value
            : siteForm.middleware.mysql.host),
        port: Number(siteForm.middleware.pgsql.port) || DEFAULT_CREDS.pgsqlPort,
        user: siteForm.middleware.pgsql.user || DEFAULT_CREDS.pgsqlUser,
        password: siteForm.middleware.pgsql.password,
      },
      redis: {
        host: siteForm.middleware.redis.host,
        port: Number(siteForm.middleware.redis.port) || 6377,
        password: siteForm.middleware.redis.password,
      },
      kafka: {
        host: siteForm.middleware.kafka.host,
        port: Number(siteForm.middleware.kafka.port) || 9092,
      },
    },
    paths: {
      workspace: siteForm.paths.workspace,
      nginxHtml: siteForm.paths.nginxHtml,
      waterwork: siteForm.paths.waterwork || undefined,
      intelligentModel: siteForm.paths.intelligentModel || undefined,
    },
  }
}

async function saveSite() {
  busy.value = true
  siteError.value = ''
  siteSaveMsg.value = ''
  try {
    const useForm = siteEditMode.value === 'form'
    if (useForm) {
      await api('/api/site/form', {
        method: 'PUT',
        body: JSON.stringify(buildFormPayload()),
      })
    } else {
      await api('/api/site', {
        method: 'PUT',
        body: JSON.stringify({ yaml: siteYaml.value }),
      })
    }
    siteSaveMsg.value = '已保存并校验通过'
    await loadSite()
  } catch (e) {
    siteError.value = e.message
  } finally {
    busy.value = false
  }
}

function fsEntryIcon(e) {
  if (e.isDir) {
    if (e.hasManifest) return 'PKG'
    if (e.hasDockerInstall) return 'DKR'
    return 'DIR'
  }
  if (e.isArchive) return (e.archiveKind || 'zip').toUpperCase()
  return 'FILE'
}

async function expandPickerDir() {
  if (!picker.current) return
  picker.expanding = true
  picker.error = ''
  picker.expandMsg = ''
  try {
    const res = await api('/api/fs/expand', {
      method: 'POST',
      body: JSON.stringify({ dir: picker.current }),
    })
    const nZip = res.zipExtracted || 0
    const nTar = res.tarUnwrapped || 0
    const nTarFiles = (res.tarFiles || []).length
    picker.expandMsg = `完成：解压 ${nZip} 个 zip，展开 ${nTar} 个 tar.zip，发现 ${nTarFiles} 个 .tar`
    await browseFS(picker.current)
  } catch (e) {
    picker.error = e.message
  } finally {
    picker.expanding = false
  }
}

async function openPicker(target, mode) {
  picker.open = true
  picker.target = target
  picker.mode = mode
  picker.error = ''
  picker.expandMsg = ''
  const start =
    (target === 'manifest' && form.manifest) ||
    (target === 'package' && form.package) ||
    (target === 'base' && form.base) ||
    (target === 'dockerPackage' && form.dockerPackage) ||
    (target === 'middlewareRoot' && fieldPaths.middlewareRoot) ||
    (target === 'platformRoot' && fieldPaths.platformRoot) ||
    (target === 'nginxDir' && fieldPaths.nginxDir) ||
    (target === 'pathsWorkspace' && siteForm.paths.workspace) ||
    (target === 'pathsNginxHtml' && siteForm.paths.nginxHtml) ||
    (target === 'waterworkDir' && siteForm.paths.waterwork) ||
    (target === 'intelligentModelDir' && siteForm.paths.intelligentModel) ||
    (target === 'nacosConfigDir' && fieldPaths.nacosConfigDir) ||
    (target === 'fetchLocal' && fetchForm.local) ||
    (target === 'scanDir' && scanForm.dir) ||
    (target === 'patchDir' && upgradeForm.patchDir) ||
    ''
  await browseFS(start)
}

async function browseFS(path) {
  picker.error = ''
  try {
    const q = new URLSearchParams({ mode: picker.mode })
    if (path) q.set('path', path)
    const data = await api('/api/fs?' + q.toString())
    picker.current = data.path || ''
    picker.parent = data.parent || ''
    picker.entries = data.entries || []
  } catch (e) {
    picker.error = e.message
    if (path) {
      await browseFS('')
    }
  }
}

function confirmPicker(path) {
  if (picker.target === 'manifest') form.manifest = path
  if (picker.target === 'package') form.package = path
  if (picker.target === 'base') form.base = path
  if (picker.target === 'dockerPackage') form.dockerPackage = path
  if (picker.target === 'middlewareRoot') {
    fieldPaths.middlewareRoot = path
    applyPathsFromMiddleware()
  }
  if (picker.target === 'platformRoot') fieldPaths.platformRoot = path
  if (picker.target === 'nginxDir') fieldPaths.nginxDir = path
  if (picker.target === 'pathsWorkspace') siteForm.paths.workspace = path
  if (picker.target === 'pathsNginxHtml') siteForm.paths.nginxHtml = path
  if (picker.target === 'waterworkDir') siteForm.paths.waterwork = path
  if (picker.target === 'intelligentModelDir') siteForm.paths.intelligentModel = path
  if (picker.target === 'nacosConfigDir') fieldPaths.nacosConfigDir = path
  if (picker.target === 'fetchLocal') fetchForm.local = path
  if (picker.target === 'scanDir') scanForm.dir = path
  if (picker.target === 'patchDir') upgradeForm.patchDir = path
  picker.open = false
}

function onFsClick(e) {
  if (e.isDir) {
    browseFS(e.path)
    return
  }
  if (e.isArchive && picker.mode === 'dir') {
    expandPickerDir()
    return
  }
  confirmPicker(e.path)
}

function onFsDblClick(e) {
  if (e.isDir && picker.mode === 'dir') confirmPicker(e.path)
}

function goWizard() {
  view.value = 'wizard'
  wizardStep.value = 0
  maxReachedStep.value = 0
  loadSite()
}

async function nextFromSite() {
  busy.value = true
  try {
    if (isLocalDocker.value) {
      if (siteEditMode.value === 'form') {
        await saveSiteFormOnly()
      }
      const summary = await api('/api/confirm-summary', {
        method: 'POST',
        body: JSON.stringify({ package: form.package }),
      })
      const ok = await askConfirm('部署前确认', summary.text || '确认进入环境体检？')
      if (!ok) return
      fieldStepDone.site = true
      applyWizardStep(1)
      precheckDone.value = false
      precheckItems.value = []
    } else {
      const ok = await prepareSiteStep()
      if (!ok) return
      applyWizardStep(1)
    }
    jobLogs.value = []
  } catch (e) {
    siteError.value = e.message
  } finally {
    busy.value = false
  }
}

function joinPath(root, name) {
  if (!root) return ''
  const sep = root.includes('\\') ? '\\' : '/'
  return root.replace(/[/\\]+$/, '') + sep + name
}

function moduleDir(phase, name) {
  const root =
    phase === 'business' ? fieldPaths.platformRoot : fieldPaths.middlewareRoot
  if (!root) return '（未配置根目录）'
  const direct = joinPath(root, name)
  const base = root.split(/[/\\]/).filter(Boolean).pop()
  const nested = joinPath(joinPath(root, base), name)
  return direct === nested ? direct : `${direct}（或 ${nested}）`
}

function completeFieldStep(key, nextStep) {
  if (!fieldStepDone[key === 'docker' ? 'site' : prevKey(key)] && key !== 'docker') {
    /* allow manual confirm */
  }
  if (key === 'docker' && !initDone.value) {
    siteError.value = '请先成功执行 Docker 安装'
    return
  }
  fieldStepDone[key] = true
  jobLogs.value = []
  applyWizardStep(nextStep)
}

function prevKey(key) {
  const order = ['site', 'docker', 'database', 'middleware', 'business', 'standalone', 'nginx']
  const i = order.indexOf(key)
  return i > 0 ? order[i - 1] : 'site'
}

async function saveSiteFormOnly() {
  const payload = buildFormPayload()
  await api('/api/site/form', { method: 'PUT', body: JSON.stringify(payload) })
}

function resolveModuleDir(phase, name) {
  const root = phase === 'business' ? fieldPaths.platformRoot : fieldPaths.middlewareRoot
  if (!root) return ''
  return joinPath(root, name)
}

async function loadTextFile(path) {
  if (!path) return
  fileEditor.loading = true
  fileEditor.msg = ''
  try {
    const data = await api('/api/fs/text?path=' + encodeURIComponent(path))
    fileEditor.path = data.path
    fileEditor.text = data.text || ''
    fileEditor.msg = `已读取 ${data.size || 0} 字节`
  } catch (e) {
    fileEditor.msg = '读取失败: ' + e.message
  } finally {
    fileEditor.loading = false
  }
}

async function saveTextFile(path) {
  const p = path || fileEditor.path
  if (!p) return
  fileEditor.loading = true
  fileEditor.msg = ''
  try {
    await api('/api/fs/text?path=' + encodeURIComponent(p), {
      method: 'PUT',
      body: JSON.stringify({ text: fileEditor.text }),
    })
    fileEditor.msg = '已保存: ' + p
  } catch (e) {
    fileEditor.msg = '保存失败: ' + e.message
  } finally {
    fileEditor.loading = false
  }
}

async function openEnvEditor(phase, name) {
  const dir = resolveModuleDir(phase, name)
  if (!dir) {
    siteError.value = '请先填写根目录'
    return
  }
  const envPath = joinPath(dir, '.env')
  fileEditor.path = envPath
  await loadTextFile(envPath)
  siteSaveMsg.value = `编辑 ${envPath}（保存后生效，需重新 compose 或重启容器）`
}

async function runModuleDeploy(phase, name, patchEnv) {
  const root =
    phase === 'business' ? fieldPaths.platformRoot : fieldPaths.middlewareRoot
  const jobKey = `${phase}-${name}`
  activeJobKey.value = jobKey
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/module/deploy', {
      method: 'POST',
      body: JSON.stringify({
        moduleRoot: root,
        moduleName: name,
        expand: true,
        load: true,
        patchEnv,
        composeUp: true,
        autoNacosImport: name === 'nacos' && !!fieldPaths.nacosConfigDir,
        nacosConfigDir: fieldPaths.nacosConfigDir || undefined,
      }),
    })
    const done = await watchJob(job.id)
    if (done.status === 'ok') {
      fieldModuleStatus[patchEnv ? 'biz-' + name : name] = 'OK'
      if (name === 'nacos' && fieldPaths.nacosConfigDir) {
        nacosImportDone.value = true
      }
    } else {
      jobLogs.value.push('ERROR: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runStandaloneDeploy(pathKey, name) {
  const moduleDir = (siteForm.paths[pathKey] || '').trim()
  if (!moduleDir) {
    jobLogs.value.push('ERROR: 未配置包目录')
    return
  }
  activeJobKey.value = 'std-' + name
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/module/deploy', {
      method: 'POST',
      body: JSON.stringify({
        moduleDir,
        expand: false,
        load: false,
        patchEnv: true,
        composeUp: true,
        composeBuild: true,
      }),
    })
    const done = await watchJob(job.id)
    if (done.status === 'ok') {
      fieldModuleStatus['std-' + name] = 'OK'
      fieldStepDone.standalone = true
    } else {
      jobLogs.value.push('ERROR: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function expandPlatformArchives(showLogs = true) {
  if (!fieldPaths.platformRoot) {
    siteError.value = '请先填写 platform 根目录'
    return false
  }
  if (showLogs) {
    busy.value = true
    jobLogs.value = []
  }
  try {
    const res = await api('/api/fs/expand', {
      method: 'POST',
      body: JSON.stringify({ dir: fieldPaths.platformRoot }),
    })
    const nZip = res.zipExtracted ?? 0
    const nTar = res.tarUnwrapped ?? 0
    const nFiles = (res.tarFiles || []).length
    const msg = `platform 解压完成：zip=${nZip} tar.zip=${nTar} 可用 .tar=${nFiles}`
    if (showLogs) jobLogs.value.push(msg)
    siteSaveMsg.value = msg
    return true
  } catch (e) {
    const partial = e.message || String(e)
    if (showLogs) jobLogs.value.push('WARN: ' + partial)
    siteSaveMsg.value = partial
    return false
  } finally {
    if (showLogs) busy.value = false
  }
}

async function patchAllBusinessEnv() {
  busy.value = true
  try {
    const res = await api('/api/module/patch-env', {
      method: 'POST',
      body: JSON.stringify({ root: fieldPaths.platformRoot }),
    })
    siteSaveMsg.value = `已更新 ${res.count || 0} 处 .env IP`
  } catch (e) {
    siteError.value = e.message
  } finally {
    busy.value = false
  }
}

function applyPathsFromMiddleware() {
  const root = fieldPaths.middlewareRoot
  if (!root) {
    siteError.value = '请先填写 middleware 根目录'
    return
  }
  const parent = root.replace(/[/\\][^/\\]+$/, '') || root
  siteForm.paths.workspace = siteForm.paths.workspace || joinPath(parent, 'waterwork')
  fieldPaths.nginxDir = fieldPaths.nginxDir || joinPath(root, 'nginx')
  siteForm.paths.nginxHtml = siteForm.paths.nginxHtml || joinPath(fieldPaths.nginxDir, 'html')
  siteSaveMsg.value = '已按 middleware 目录填充 paths（nginxHtml = nginx/html）'
}

async function runNginxPatch(expandHtml = true) {
  activeJobKey.value = expandHtml ? 'nginx-patch' : 'nginx-patch-only'
  busy.value = true
  jobLogs.value = []
  nginxPatchDone.value = false
  try {
    const job = await api('/api/nginx/patch', {
      method: 'POST',
      body: JSON.stringify({
        nginxDir: fieldPaths.nginxDir,
        gatewayIp: fieldPaths.gatewayIP || primaryNodeIP.value,
        appIp: fieldPaths.appIP || undefined,
        graphIp: fieldPaths.graphIP || undefined,
        expandHtml: !!expandHtml,
        composeUp: true,
      }),
    })
    const done = await watchJob(job.id)
    nginxPatchDone.value = done.status === 'ok'
    if (done.status === 'ok') fieldStepDone.nginx = true
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runNacosImport() {
  activeJobKey.value = 'nacos-import'
  busy.value = true
  jobLogs.value = []
  nacosImportDone.value = false
  try {
    const job = await api('/api/nacos/import', {
      method: 'POST',
      body: JSON.stringify({ configDir: fieldPaths.nacosConfigDir }),
    })
    const done = await watchJob(job.id)
    nacosImportDone.value = done.status === 'ok'
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runFirewallStart() {
  activeJobKey.value = 'firewall-start'
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/firewall/start', { method: 'POST', body: '{}' })
    const done = await watchJob(job.id)
    if (done.status === 'ok') await loadFirewallPorts()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function loadFirewallPorts() {
  try {
    const q = new URLSearchParams()
    if (form.manifest) q.set('manifest', form.manifest)
    if (form.package) q.set('package', form.package)
    const data = await api('/api/firewall/ports?' + q.toString())
    firewallPorts.value = data.ports || []
    firewallType.value = data.firewall || ''
    firewallRunning.value = !!data.firewallRunning
    firewallDetail.value = data.firewallDetail || ''
  } catch (e) {
    siteError.value = e.message
  }
}

async function runFirewallOpen() {
  activeJobKey.value = 'firewall-open'
  busy.value = true
  jobLogs.value = []
  firewallOpenDone.value = false
  try {
    const job = await api('/api/firewall/ports', {
      method: 'POST',
      body: JSON.stringify({
        manifest: form.manifest || undefined,
        package: form.package || undefined,
        autoStart: true,
      }),
    })
    const done = await watchJob(job.id)
    firewallOpenDone.value = done.status === 'ok'
    if (done.status === 'ok') fieldStepDone.firewall = true
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

function goStatus() {
  view.value = 'status'
  loadStatus()
}

function watchJob(id) {
  return new Promise((resolve) => {
    if (jobWs) jobWs.close()
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    jobWs = new WebSocket(`${proto}://${location.host}/api/ws/job?id=${id}`)
    jobWs.onmessage = (ev) => {
      const job = JSON.parse(ev.data)
      jobLogs.value = job.logs || []
      const smoke = job.result?.smoke || job.result?.Smoke
      if (Array.isArray(smoke)) {
        smokeRows.value = smoke.map((s) => ({
          name: s.name || s.Name,
          port: s.port || s.Port,
          ok: s.ok ?? s.OK,
          message: s.message || s.Message,
        }))
      }
      if (job.status === 'ok' || job.status === 'fail') {
        jobWs.close()
        resolve(job)
      }
    }
    jobWs.onerror = () => resolve({ status: 'fail', message: 'websocket error', result: null })
  })
}

async function runPrecheck() {
  busy.value = true
  jobLogs.value = []
  precheckDone.value = false
  try {
    const job = await api('/api/precheck', {
      method: 'POST',
      body: JSON.stringify({ manifest: form.manifest }),
    })
    const done = await watchJob(job.id)
    const rep = done.result
    precheckItems.value = rep?.items || []
    precheckBlocked.value = !!rep?.hasRed || done.status === 'fail'
    precheckDone.value = true
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function runInit() {
  activeJobKey.value = 'init'
  busy.value = true
  jobLogs.value = []
  initDone.value = false
  try {
    const job = await api('/api/init', {
      method: 'POST',
      body: JSON.stringify({
        base: form.base,
        dockerPackage: form.dockerPackage,
        manifest: form.manifest,
        local: !isMultiNode.value,
        sshPassword: sshCreds.password || undefined,
        sshKeyPath: sshCreds.keyPath || undefined,
      }),
    })
    const done = await watchJob(job.id)
    initDone.value = done.status === 'ok'
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runDeploy(dryRun) {
  busy.value = true
  jobLogs.value = []
  smokeRows.value = []
  deployDone.value = false
  try {
    const job = await api('/api/deploy', {
      method: 'POST',
      body: JSON.stringify({
        package: form.package,
        dryRun,
        operator: settings.operator,
        scenario: settings.scenario,
      }),
    })
    const done = await watchJob(job.id)
    deployDone.value = done.status === 'ok'
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
    else await loadLatest()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function loadPreview() {
  preview.value = null
  try {
    preview.value = await api('/api/preview', {
      method: 'POST',
      body: JSON.stringify({ package: form.package }),
    })
  } catch (e) {
    jobLogs.value = ['ERROR: ' + e.message]
  }
}

async function openStatus() {
  view.value = 'status'
  await loadStatus()
}

async function loadStatus() {
  try {
    const data = await api('/api/status')
    services.value = data.services || []
    statusWarning.value = data.warning || ''
  } catch (e) {
    statusWarning.value = e.message
    services.value = []
  }
}

async function openHistory() {
  view.value = 'history'
  await loadHistory()
}

async function openFetch() {
  view.value = 'fetch'
  jobLogs.value = []
  fetchResultDir.value = ''
  await loadPackages()
}

function openReport(id) {
  reportId.value = id || ''
  view.value = 'report'
}

function exportDeliveries() {
  window.location = '/api/deliveries/export'
}

async function downloadDiag() {
  busy.value = true
  try {
    const res = await fetch('/api/diag')
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || res.statusText)
    }
    const blob = await res.blob()
    const cd = res.headers.get('Content-Disposition') || ''
    const match = /filename=([^;]+)/i.exec(cd)
    const name = match ? match[1].trim().replace(/"/g, '') : 'wpgctl-diag.tar.gz'
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    alert('诊断包下载失败: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function openUpgrade() {
  if (!latestVersion.value) return
  view.value = 'upgrade'
  jobLogs.value = []
  await loadLatest()
}

async function runFetch() {
  busy.value = true
  jobLogs.value = []
  fetchResultDir.value = ''
  try {
    const job = await api('/api/fetch', {
      method: 'POST',
      body: JSON.stringify({
        name: fetchForm.name,
        from: fetchForm.from,
        local: fetchForm.local,
      }),
    })
    const done = await watchJob(job.id)
    if (done.status === 'ok') {
      fetchResultDir.value = done.result?.packageDir || done.result?.PackageDir || ''
      await loadPackages()
    } else {
      jobLogs.value.push('ERROR: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

function applyHostToForm(host) {
  if (!host) return
  if (siteForm.nodes[0]) siteForm.nodes[0].ip = host
  if (deployTopology.value === 'multi' && siteForm.nodes.length > 1) {
    applyMiddlewareFromRoles()
    return
  }
  siteForm.middleware.nacos.host = host
  if (!siteForm.middleware.mysql.disabled) {
    siteForm.middleware.mysql.host = host
  }
  siteForm.middleware.pgsql.host = host
  siteForm.middleware.redis.host = host
  siteForm.middleware.kafka.host = host
}

async function fillHostsAuto() {
  hostFillMsg.value = ''
  try {
    const info = await api('/api/hostinfo')
    const loopback = info.loopback || '127.0.0.1'
    const preferred = info.preferred || loopback
    const addrs = Array.isArray(info.addresses) ? info.addresses.slice() : []
    if (!addrs.includes(loopback)) addrs.unshift(loopback)
    if (preferred && !addrs.includes(preferred)) addrs.unshift(preferred)

    // 本机 Docker 默认 127.0.0.1；现场交付用探测到的网卡 IP
    const pick = isLocalDocker.value ? loopback : preferred
    hostCandidates.value = addrs
    selectedHost.value = pick
    applyHostToForm(pick)

    const hostHint = info.hostname ? `（${info.hostname}）` : ''
    if (isLocalDocker.value) {
      hostFillMsg.value = `已填入 ${pick}${hostHint}；如需局域网 IP 可在下拉框切换`
    } else {
      const sync = siteForm.middleware.mysql.disabled
        ? 'Nacos/PgSQL/Redis/Kafka Host'
        : 'Nacos/MySQL/PgSQL/Redis/Kafka Host'
      hostFillMsg.value = `已填入 ${pick}${hostHint}；已同步到 ${sync}`
    }
  } catch (e) {
    hostFillMsg.value = '获取失败: ' + e.message
  }
}

function applySelectedHost() {
  applyHostToForm(selectedHost.value)
  hostFillMsg.value = `已切换为 ${selectedHost.value}`
}

async function runScan(write) {
  busy.value = true
  scanError.value = ''
  scanMsg.value = ''
  try {
    const data = await api('/api/packages/scan', {
      method: 'POST',
      body: JSON.stringify({
        dir: scanForm.dir,
        kind: scanForm.kind || 'base',
        version: scanForm.version,
        write: !!write,
      }),
    })
    scanImages.value = data.images || []
    scanYaml.value = data.yaml || ''
    if (data.warnings?.length) {
      scanMsg.value = `完成，有 ${data.warnings.length} 条提示` + (write && data.manifestPath ? `；已写入 ${data.manifestPath}` : '')
    } else if (write && data.manifestPath) {
      scanMsg.value = `已写入 ${data.manifestPath}`
    } else {
      scanMsg.value = `预览完成，共 ${data.count || scanImages.value.length} 个服务`
    }
    if (write) await loadPackages()
  } catch (e) {
    scanError.value = e.message
  } finally {
    busy.value = false
  }
}

function useFetchedPackage() {
  if (fetchResultDir.value) {
    usePackagePath(fetchResultDir.value)
  }
}

function usePackagePath(path) {
  form.package = path
  goWizard()
}

async function runUpgrade() {
  const ok = await askConfirm('确认升级', `确认由 ${settings.operator || 'operator'} 执行？`)
  if (!ok) return
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/upgrade', {
      method: 'POST',
      body: JSON.stringify({ patchDir: upgradeForm.patchDir, yes: true }),
    })
    const done = await watchJob(job.id)
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
    else await openUpgrade()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function runRollback() {
  const ok = await askConfirm('确认回滚', `确认由 ${settings.operator || 'operator'} 执行？`)
  if (!ok) return
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/rollback', {
      method: 'POST',
      body: JSON.stringify({ to: upgradeForm.rollbackTo }),
    })
    const done = await watchJob(job.id)
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
    else await openUpgrade()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function loadHistory() {
  try {
    deployments.value = await api('/api/deployments')
  } catch {
    deployments.value = []
  }
}

function startLogs() {
  stopLogs()
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  logWs = new WebSocket(`${proto}://${location.host}/api/ws/logs?service=${encodeURIComponent(logService.value)}`)
  logWs.onmessage = (ev) => {
    const data = JSON.parse(ev.data)
    if (data.error) liveLogs.value = data.error
    else liveLogs.value = data.logs || ''
  }
}

function stopLogs() {
  if (logWs) {
    logWs.close()
    logWs = null
  }
}

function formatTime(t) {
  if (!t) return '—'
  try {
    return new Date(t).toLocaleString()
  } catch {
    return t
  }
}

watch(wizardStep, (step, prev) => {
  if (step === 4 && prev !== 4 && !isLocalDocker.value && fieldPaths.platformRoot) {
    expandPlatformArchives(true)
  }
})

onMounted(async () => {
  try {
    const h = await api('/api/health')
    runtimeOS.value = h.os || ''
    runtimeArch.value = h.arch || ''
    dockerOk.value = h.dockerOk
    dockerMsg.value = h.dockerMsg || ''
    deployHint.value = h.deployHint || ''
  } catch {
    /* ignore */
  }
  await Promise.all([loadSettings(), loadSite(), loadPackages(), loadLatest()])
})
onUnmounted(() => {
  stopLogs()
  if (jobWs) jobWs.close()
})
</script>
