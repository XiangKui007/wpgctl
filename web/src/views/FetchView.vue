<script>
/** 包中心：扫描 manifest、本地包列表、拉包/导入。 */
import { useConsole } from '@/composables/useConsole.js'
import YamlEditor from '@/components/YamlEditor.vue'

export default {
  name: 'FetchView',
  components: { YamlEditor },
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel">
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
            <label class="req">包目录</label>
            <div class="path-row">
              <el-input v-model="scanForm.dir" placeholder="例如 D:/package/middleware/middleware" />
              <el-button type="primary" plain size="small" @click="openPicker('scanDir', 'dir')">浏览</el-button>
            </div>
          </div>
          <div class="field">
            <label>kind</label>
            <el-input v-model="scanForm.kind" placeholder="base 或 release" />
          </div>
          <div class="field">
            <label>包版本（可选）</label>
            <el-input v-model="scanForm.version" placeholder="默认 1.0.0" />
          </div>
        </div>
        <div class="actions" style="margin-top:0">
          <el-button type="primary" plain @click="runScan(false)" :disabled="busy || !scanForm.dir">预览</el-button>
          <el-button type="success" plain @click="runScan(true)" :disabled="busy || !scanForm.dir">生成并写入目录</el-button>
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
          <el-button type="primary" plain size="small" @click="loadPackages">刷新</el-button>
        </div>
        <el-table v-if="packagesList.length" :data="packagesList" stripe style="width:100%">
          <el-table-column prop="name" label="名称" min-width="140" />
          <el-table-column label="类型" width="100">
            <template #default="{ row }">{{ row.kind || '—' }}</template>
          </el-table-column>
          <el-table-column label="版本" width="110">
            <template #default="{ row }">{{ row.version || '—' }}</template>
          </el-table-column>
          <el-table-column label="路径" min-width="220">
            <template #default="{ row }">{{ settings.privacyMode ? '******' : row.path }}</template>
          </el-table-column>
          <el-table-column label="" width="120">
            <template #default="{ row }">
              <el-button type="success" plain size="small" @click="usePackagePath(row.path)">用于部署</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="本地尚无带 manifest.yaml 的包。" />
      </div>
      <div class="surface">
        <h3 style="margin:0 0 1rem;font-family:var(--font-display)">拉包 / 本地导入</h3>
        <div class="field-grid">
          <div class="field">
            <label>包名（远程拉取）</label>
            <el-input v-model="fetchForm.name" placeholder="release-4.0.2 或 patch-4.0.3" />
          </div>
          <div class="field">
            <label>拉包基址 URL（可选）</label>
            <el-input v-model="fetchForm.from" placeholder="https://packages.example.com/packages" />
          </div>
          <div class="field full">
            <label>或：本地分卷 / 完整包目录</label>
            <div class="path-row">
              <el-input v-model="fetchForm.local" placeholder="包目录，或含 .zip / .tar.gz 的目录" />
              <el-button type="primary" plain size="small" @click="openPicker('fetchLocal', 'dir')">浏览</el-button>
            </div>
            <p class="hint-banner" style="margin-top:0.5rem;margin-bottom:0">
              目录内若是 <code>.zip</code> / <code>.tar.zip</code>，在浏览窗口点「解压 ZIP / 展开 TAR」；本地导入也会自动尝试解压后再合并 tar.gz。
            </p>
          </div>
        </div>
        <div class="actions">
          <el-button type="primary" @click="runFetch" :disabled="busy || (!fetchForm.name && !fetchForm.local)">
            {{ busy ? '进行中…' : '开始拉包 / 导入' }}
          </el-button>
        </div>
        <div class="log-box" v-if="jobLogs.length">
          <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
        </div>
        <p v-if="fetchResultDir" class="muted" style="color:var(--ok);margin-top:1rem">
          包已就绪：{{ settings.privacyMode ? '******' : fetchResultDir }}
          <el-button type="success" plain size="small" style="margin-left:0.5rem" @click="useFetchedPackage">用于部署向导</el-button>
        </p>
      </div>
    </section>
</template>
