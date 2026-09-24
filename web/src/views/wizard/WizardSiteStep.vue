<script>
/** 向导 ① 节点配置（表单 5 小步 / YAML）。 */
import { useConsole } from '@/composables/useConsole.js'
import YamlEditor from '@/components/YamlEditor.vue'

export default {
  name: 'WizardSiteStep',
  components: { YamlEditor },
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <el-radio-group :model-value="siteEditMode" size="small" style="margin:0 0 1.1rem" @change="switchSiteMode">
            <el-radio-button value="form">表单</el-radio-button>
            <el-radio-button value="yaml">YAML</el-radio-button>
          </el-radio-group>

          <div v-if="siteEditMode === 'form'">
            <ol class="sub-steps" aria-label="节点配置分步">
              <li
                v-for="(s, i) in siteSubSteps"
                :key="s.key"
                class="sub-step"
                :class="{
                  active: siteSubStep === i,
                  done: i < siteSubStep,
                  locked: i > siteSubStepReached,
                }"
                @click="goSiteSubStep(i)"
              >
                <span class="sub-step-idx">{{ i + 1 }}</span>
                <span class="sub-step-title">{{ s.title }}</span>
              </li>
            </ol>
            <div class="sub-step-head">
              <strong>第 {{ siteSubStep + 1 }} 小步 / {{ siteSubSteps.length }}：{{ currentSiteSubStep.title }}</strong>
              <p class="muted">{{ currentSiteSubStep.desc }}</p>
            </div>

            <!-- 1. 项目信息 -->
            <div v-show="currentSiteSubStep.key === 'project'" class="field-grid">
            <div class="field">
              <label class="req">项目名称</label>
              <el-input v-model="siteForm.site.name" placeholder="水厂/项目显示名，如：某某市政水厂" />
            </div>
            <div class="field">
              <label class="req">项目编码</label>
              <el-input v-model="siteForm.site.code" placeholder="英文/数字，如 wpg-demo；用于 site.yaml 标识" />
            </div>
            </div>

            <!-- 3. 中间件连接 -->
            <div v-show="currentSiteSubStep.key === 'middleware'" class="field-grid">
            <div class="field full">
              <div class="host-fill-bar">
                <el-button type="primary" plain size="small" @click="applyDefaultCreds(true)" :disabled="busy">
                  填充默认账号密码
                </el-button>
                <span v-if="hostFillMsg" class="muted host-fill-msg">{{ hostFillMsg }}</span>
              </div>
              <p class="hint-banner" style="margin:0.45rem 0 0">
                标准账号已填。Host 已按机器规划同步。
              </p>
            </div>
            <div class="field">
              <label class="req">Nacos Host</label>
              <el-input v-model="siteForm.middleware.nacos.host" />
            </div>
            <div class="field">
              <label class="req">Nacos 用户 / 密码</label>
              <div class="path-row credential-row">
                <el-input v-model="siteForm.middleware.nacos.username" placeholder="nacos" />
                <el-input v-model="siteForm.middleware.nacos.password" type="password" show-password autocomplete="new-password" placeholder="标准密码（已预设）" />
              </div>
            </div>
            <div class="field full">
              <el-checkbox v-model="siteForm.middleware.mysql.disabled">
                本版本不依赖 MySQL（仅 PgSQL，跳过 MySQL 部署与连通检查）
              </el-checkbox>
            </div>
            <template v-if="!siteForm.middleware.mysql.disabled">
              <div class="field">
                <label class="req">MySQL Host</label>
                <el-input v-model="siteForm.middleware.mysql.host" />
              </div>
              <div class="field">
                <label class="req">MySQL 用户 / 密码</label>
                <div class="path-row credential-row">
                  <el-input v-model="siteForm.middleware.mysql.user" placeholder="wpg" />
                  <el-input v-model="siteForm.middleware.mysql.password" type="password" show-password autocomplete="new-password" placeholder="标准密码（已预设）" />
                </div>
              </div>
            </template>
            <div class="field">
              <label class="req">Redis Host</label>
              <el-input v-model="siteForm.middleware.redis.host" />
            </div>
            <div class="field">
              <label class="req">Redis 密码</label>
              <el-input v-model="siteForm.middleware.redis.password" type="password" show-password autocomplete="new-password" placeholder="标准密码（已预设）" />
            </div>
            <div class="field">
              <label class="req">PgSQL Host</label>
              <el-input v-model="siteForm.middleware.pgsql.host" />
            </div>
            <div class="field">
              <label class="req">PgSQL 用户 / 密码</label>
              <div class="path-row credential-row">
                <el-input v-model="siteForm.middleware.pgsql.user" placeholder="wpg" />
                <el-input v-model="siteForm.middleware.pgsql.password" type="password" show-password autocomplete="new-password" placeholder="标准密码（已预设）" />
              </div>
            </div>
            <div class="field">
              <label class="req">Kafka Host</label>
              <el-input v-model="siteForm.middleware.kafka.host" />
            </div>
            </div>

            <!-- 2. 机器规划 -->
            <div v-show="currentSiteSubStep.key === 'machines'" class="field-grid">
            <div class="field full">
              <label>部署方式</label>
              <el-radio-group :model-value="deployTopology" size="small" @change="setDeployTopology">
                <el-radio-button value="multi">多台机器（推荐）</el-radio-button>
                <el-radio-button value="single">一台机器</el-radio-button>
              </el-radio-group>
              <p v-if="deployTopology === 'single'" class="muted" style="margin:0.45rem 0 0;font-size:0.85rem">
                全部服务跑在这一台。
              </p>
              <p v-else class="muted" style="margin:0.45rem 0 0;font-size:0.85rem">
                第一台为主控；从机填 IP 与 SSH。
              </p>
            </div>
            <div v-if="deployTopology === 'single'" class="field">
              <label class="req">机器 IP</label>
              <div class="path-row">
                <el-input v-model="siteForm.nodes[0].ip" placeholder="现场主控机 IP" />
                <span
                  v-if="nodeDockerLabel(siteForm.nodes[0])"
                  class="badge"
                  :class="nodeDockerOf(siteForm.nodes[0]).ok ? 'green' : 'yellow'"
                  :title="nodeDockerOf(siteForm.nodes[0]).message || ''"
                >{{ nodeDockerLabel(siteForm.nodes[0]) }}</span>
              </div>
            </div>
            <div v-if="deployTopology === 'multi'" class="field full">
              <div class="node-section-head">
                <div>
                  <strong>1. 添加机器</strong>
                  <p class="muted">第一台为主控机。</p>
                </div>
                <div class="actions compact">
                  <el-button type="primary" plain size="small" :loading="nodeDockerBusy" @click="refreshNodeDocker">
                    {{ nodeDockerBusy ? '检查 Docker…' : '检查各机 Docker' }}
                  </el-button>
                  <el-button type="primary" plain size="small" @click="applyDualNodeTemplate">常用双机方案</el-button>
                  <el-button type="primary" plain size="small" @click="addNode">+ 添加机器</el-button>
                </div>
              </div>
              <div class="host-fill-bar">
                <el-button type="primary" plain size="small" @click="fillHostsAuto" :disabled="busy">
                  自动获取本机 IP
                </el-button>
                <el-select
                  v-if="hostOptions.length > 1"
                  v-model="selectedHost"
                  size="small"
                  style="width: 12rem"
                  @change="applySelectedHost"
                >
                  <el-option v-for="ip in hostOptions" :key="ip" :label="ip" :value="ip" />
                </el-select>
                <span v-if="hostFillMsg" class="muted host-fill-msg">{{ hostFillMsg }}</span>
              </div>
              <div class="node-card-grid">
                <article v-for="(n, idx) in siteForm.nodes" :key="idx" class="node-card">
                  <div class="node-card-title">
                    <span class="node-number">{{ idx + 1 }}</span>
                    <strong>{{ n.name || `机器 ${idx + 1}` }}</strong>
                    <span v-if="idx === 0" class="badge green">当前主控机</span>
                    <span
                      v-if="nodeDockerLabel(n)"
                      class="badge"
                      :class="nodeDockerOf(n).ok ? 'green' : 'yellow'"
                      :title="nodeDockerOf(n).message || nodeDockerLabel(n)"
                    >{{ nodeDockerLabel(n) }}</span>
                    <el-button
                      v-if="idx > 0"
                      type="danger"
                      plain
                      size="small"
                      title="删除机器"
                      @click="removeNode(idx)"
                    >删除</el-button>
                  </div>
                  <div class="node-card-fields">
                    <label>
                      <span class="req">机器名称</span>
                      <el-input v-model="n.name" :placeholder="idx === 0 ? '主控机' : `node${idx + 1}`" />
                    </label>
                    <label>
                      <span class="req">IP 地址</span>
                      <el-input v-model="n.ip" placeholder="192.168.1.10" />
                    </label>
                    <label>
                      <span class="req">SSH 用户</span>
                      <el-input v-model="n.sshUser" placeholder="root" />
                    </label>
                    <label class="ssh-port">
                      <span>端口</span>
                      <el-input-number v-model.number="n.sshPort" min="1" max="65535" controls-position="right" />
                    </label>
                    <label class="ssh-pass">
                      <span>SSH 密码</span>
                      <el-input v-model="n.sshPassword" type="password" show-password autocomplete="new-password" placeholder="写入 site.yaml，下次回填" />
                    </label>
                  </div>
                  <div class="node-service-summary">
                    <span class="muted">已分配</span>
                    <strong>{{ assignedServicesForNode(idx).length }}</strong>
                    <span class="muted">项服务</span>
                  </div>
                </article>
              </div>

              <div class="node-section-head service-section-head">
                <div>
                  <strong>2. 分配服务</strong>
                  <p class="muted">每项服务选一台。Nginx 只在主控机。</p>
                </div>
                <el-button type="primary" plain size="small" @click="assignAllToPrimary">全部放到主控机</el-button>
              </div>
              <div class="service-assignment">
                <section v-for="group in NODE_SERVICE_GROUPS" :key="group.id" class="service-group">
                  <div class="service-group-title">
                    <strong>{{ group.label }}</strong>
                    <span>{{ group.hint }}</span>
                  </div>
                  <div class="service-assign-list">
                    <label v-for="svc in group.services" :key="svc.id" class="service-assign-row">
                      <span>{{ svc.label }}</span>
                      <el-select
                        v-if="svc.id === 'nginx'"
                        v-model="serviceAssignments.nginx"
                        size="small"
                        style="width: 14rem"
                        @change="syncServiceAssignments"
                      >
                        <el-option :value="-1" label="不部署" />
                        <el-option :value="0" :label="primaryNodeLabel" />
                      </el-select>
                      <el-select
                        v-else
                        v-model="serviceAssignments[svc.id]"
                        size="small"
                        style="width: 14rem"
                        @change="syncServiceAssignments"
                      >
                        <el-option :value="-1" label="不部署" />
                        <el-option
                          v-for="(n, idx) in siteForm.nodes"
                          :key="idx"
                          :value="idx"
                          :label="`${n.name || '机器 ' + (idx + 1)} · ${n.ip || '未填 IP'}`"
                        />
                      </el-select>
                    </label>
                  </div>
                </section>
              </div>

              <div class="field-grid" style="margin-top:0.75rem">
                <div class="field">
                  <label>未单独填写时使用的 SSH 密码</label>
                  <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各机器密码相同时填写，会写入 site.yaml" show-password />
                </div>
                <div class="field">
                  <label>或 SSH 私钥路径（仅本次使用）</label>
                  <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
                </div>
              </div>
            </div>
            <div v-if="deployTopology === 'single'" class="field full">
              <div class="host-fill-bar">
                <el-button type="primary" plain size="small" @click="fillHostsAuto" :disabled="busy">
                  自动获取本机 IP
                </el-button>
                <el-select
                  v-if="hostOptions.length > 1"
                  v-model="selectedHost"
                  size="small"
                  style="width: 12rem"
                  @change="applySelectedHost"
                >
                  <el-option v-for="ip in hostOptions" :key="ip" :label="ip" :value="ip" />
                </el-select>
                <span v-if="hostFillMsg" class="muted host-fill-msg">{{ hostFillMsg }}</span>
              </div>
              <p class="muted" style="margin:0.45rem 0 0;font-size:0.85rem">多网卡时在下拉框切换。</p>
            </div>
            </div>

            <!-- 5. 确认保存 -->
            <div v-show="currentSiteSubStep.key === 'confirm'" class="site-summary">
              <section class="summary-block">
                <header>
                  <strong>项目</strong>
                  <el-button type="primary" link class="link-btn" @click="goSiteSubStepByKey('project')">修改</el-button>
                </header>
                <dl>
                  <dt>名称</dt><dd>{{ siteForm.site.name || '—' }}</dd>
                  <dt>编码</dt><dd>{{ siteForm.site.code || '—' }}</dd>
                </dl>
              </section>
              <section class="summary-block">
                <header>
                  <strong>机器规划</strong>
                  <el-button type="primary" link class="link-btn" @click="goSiteSubStepByKey('machines')">修改</el-button>
                </header>
                <dl v-if="deployTopology === 'single'">
                  <dt>部署方式</dt><dd>一台机器（所有服务）</dd>
                  <dt>IP</dt><dd>{{ siteForm.nodes[0]?.ip || '—' }}</dd>
                </dl>
                <ul v-else class="summary-nodes">
                  <li v-for="(n, idx) in siteForm.nodes" :key="idx">
                    <span class="node-number">{{ idx + 1 }}</span>
                    <strong>{{ n.name || `机器 ${idx + 1}` }}</strong>
                    <code>{{ n.ip || '未填 IP' }}</code>
                    <span class="muted">{{ n.sshUser || 'root' }}@:{{ n.sshPort || 22 }}</span>
                    <span class="muted">· {{ assignedServiceLabels(idx).join('、') || '未分配服务' }}</span>
                  </li>
                </ul>
              </section>
              <section class="summary-block">
                <header>
                  <strong>中间件连接</strong>
                  <el-button type="primary" link class="link-btn" @click="goSiteSubStepByKey('middleware')">修改</el-button>
                </header>
                <dl>
                  <dt>Nacos</dt><dd>{{ siteForm.middleware.nacos.host || '—' }}</dd>
                  <dt>MySQL</dt><dd>{{ siteForm.middleware.mysql.disabled ? '不使用' : (siteForm.middleware.mysql.host || '—') }}</dd>
                  <dt>Redis</dt><dd>{{ siteForm.middleware.redis.host || '—' }}</dd>
                  <dt>PgSQL</dt><dd>{{ siteForm.middleware.pgsql.host || '—' }}</dd>
                  <dt>Kafka</dt><dd>{{ siteForm.middleware.kafka.host || '—' }}</dd>
                </dl>
              </section>
              <section class="summary-block">
                <header>
                  <strong>目录与安装包</strong>
                  <el-button type="primary" link class="link-btn" @click="goSiteSubStepByKey('paths')">修改</el-button>
                </header>
                <dl>
                    <dt>middleware 根目录</dt><dd>{{ fieldPaths.middlewareRoot || '—' }}</dd>
                    <dt>platform 根目录</dt><dd>{{ fieldPaths.platformRoot || '—' }}</dd>
                    <dt>Docker 离线包</dt><dd>{{ form.dockerPackage || '—（② 安装 Docker 前需填写）' }}</dd>
                </dl>
              </section>
            </div>
          </div>

          <div v-else class="field full" style="margin-bottom: 1rem">
            <label>site.yaml 内容（YAML 高亮编辑）</label>
            <YamlEditor v-model="siteYaml" />
          </div>

          <!-- 4. 目录与安装包（fieldPaths 不在 site.yaml 内，YAML 模式下也需填写） -->
          <div v-show="siteEditMode === 'yaml' || currentSiteSubStep.key === 'paths'" class="field-grid">
            <div v-if="siteEditMode === 'yaml'" class="field full">
              <label>向导工作目录（不写入 site.yaml，仅本工具使用）</label>
            </div>
            <p class="hint-banner field full" style="margin:0 0 0.5rem">
              点 zip 不会解压；需要时点该行「解压此包」。
            </p>
            <div class="field full">
              <label class="req">middleware 根目录</label>
              <div class="path-row">
                <el-input v-model="fieldPaths.middlewareRoot" placeholder="如 /workspace/middle 或 /workspace/middleware" />
                <el-button type="primary" plain size="small" @click="openPicker('middlewareRoot', 'dir')">浏览</el-button>
              </div>
              <p class="muted" style="margin:0.35rem 0 0">含 mysql / pgsql 的目录，或其外层。</p>
            </div>
            <div class="field full">
              <label>platform 根目录</label>
              <div class="path-row">
                <el-input v-model="fieldPaths.platformRoot" placeholder="如 /workspace/platform（含 public/device）" />
                <el-button type="primary" plain size="small" @click="openPicker('platformRoot', 'dir')">浏览</el-button>
              </div>
              <p class="muted" style="margin:0.35rem 0 0">含 public / device 的目录。</p>
            </div>
            <div class="field full">
              <label>市政水厂包目录（可选）</label>
              <div class="path-row">
                <el-input
                  v-model="siteForm.paths.waterwork"
                  placeholder="独立包目录，非 platform 子目录" />
                <el-button type="primary" plain size="small" @click="openPicker('waterworkDir', 'dir')">浏览</el-button>
              </div>
            </div>
            <div class="field full">
              <label>模型服务包目录（可选）</label>
              <div class="path-row">
                <el-input
                  v-model="siteForm.paths.intelligentModel"
                  placeholder="独立包目录" />
                <el-button type="primary" plain size="small" @click="openPicker('intelligentModelDir', 'dir')">浏览</el-button>
              </div>
            </div>
            <div class="field full">
              <label class="req">Docker 离线安装目录（Linux 首次必装）</label>
              <div class="path-row">
                <el-input
                  v-model="form.dockerPackage"
                  placeholder="如 middleware/docker_package/docker_package" />
                <el-button type="primary" plain size="small" @click="openPicker('dockerPackage', 'dir')">浏览</el-button>
              </div>
            </div>
          </div>

          <p v-if="siteSaveMsg" class="muted" style="color: var(--ok)">{{ siteSaveMsg }}</p>
          <p v-else-if="siteEditMode === 'form'" class="muted" style="margin:0.35rem 0 0">
            项目名、机器和中间件填齐后会自动写入 site.yaml，刷新浏览器不会丢。
          </p>
          <p v-if="siteError && siteEditMode === 'form'" class="wizard-alert muted sub-step-error">{{ siteError }}</p>
          <div class="actions sub-step-nav" style="margin-top: 0.5rem">
            <template v-if="siteEditMode === 'form'">
              <el-button v-if="siteSubStep > 0" plain @click="prevSiteSubStep" :disabled="busy">
                上一步
              </el-button>
              <el-button type="primary"
                v-if="siteSubStep < siteSubSteps.length - 1"
                @click="nextSiteSubStep"
                :disabled="busy"
              >
                下一步
              </el-button>
              <template v-else>
                <el-button type="primary" plain @click="saveSite" :disabled="busy">保存 site.yaml</el-button>
                <el-button plain @click="loadSite" :disabled="busy">重新加载</el-button>
                <el-button type="primary" @click="nextFromSite" :disabled="busy">
                  保存并进入 ② 安装 Docker
                </el-button>
              </template>
            </template>
            <template v-else>
              <el-button type="primary" plain @click="saveSite" :disabled="busy">保存 site.yaml</el-button>
              <el-button plain @click="loadSite" :disabled="busy">重新加载</el-button>
              <el-button type="primary" @click="nextFromSite" :disabled="busy">
                保存并进入 ② 安装 Docker
              </el-button>
            </template>
          </div>
        </div>
</template>
