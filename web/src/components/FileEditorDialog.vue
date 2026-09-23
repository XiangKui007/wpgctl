<script>
/**
 * .env / nginx conf 弹框编辑：打开即读盘，保存写回，不打断向导步骤。
 */
import { computed, ref, watch } from 'vue'
import { useConsole } from '@/composables/useConsole.js'
import CodeEditor from './CodeEditor.vue'

export default {
  name: 'FileEditorDialog',
  components: { CodeEditor },
  setup() {
    const ctx = useConsole()
    const editorReady = ref(false)
    const open = ref(false)
    watch(
      () => ctx.fileEditor.path,
      (p) => {
        if (p) open.value = true
      },
    )
    const fileName = computed(() => {
      const p = String(ctx.fileEditor.path || '').replace(/\\/g, '/')
      const i = p.lastIndexOf('/')
      return i >= 0 ? p.slice(i + 1) : p || '配置文件'
    })
    const title = computed(() => {
      const name = fileName.value
      if (name.endsWith('.env')) return '编辑 .env'
      if (name.endsWith('.conf')) return '编辑 Nginx 配置'
      return '编辑 ' + name
    })
    /** 候选按钮只显示「父目录/文件名」，如 giscenter/.env。 */
    function candidateLabel(p) {
      const parts = String(p || '').replace(/\\/g, '/').split('/').filter(Boolean)
      return parts.slice(-2).join('/')
    }
    function onOpened() {
      editorReady.value = true
    }
    function onClosed() {
      editorReady.value = false
      ctx.closeFileEditor()
    }
    return {
      ...ctx,
      open,
      fileName,
      title,
      editorReady,
      candidateLabel,
      onOpened,
      onClosed,
    }
  },
}
</script>

<template>
  <el-dialog
    v-model="open"
    class="file-editor-dialog"
    :title="title"
    width="920px"
    top="6vh"
    append-to-body
    destroy-on-close
    :close-on-click-modal="false"
    @opened="onOpened"
    @closed="onClosed"
  >
    <p class="file-editor-path muted">
      <code>{{ fileEditor.path }}</code>
    </p>
    <!-- GIS 等模块下有多份同名 .env（giscenter / gisdefault），在这里切换，各自单独保存 -->
    <div v-if="fileEditor.candidates.length > 1" class="file-editor-candidates">
      <span class="muted">该模块有 {{ fileEditor.candidates.length }} 份同名文件：</span>
      <el-radio-group :model-value="fileEditor.path" size="small" @change="switchFileCandidate">
        <el-radio-button v-for="c in fileEditor.candidates" :key="c" :value="c" :title="c">
          {{ candidateLabel(c) }}
        </el-radio-button>
      </el-radio-group>
    </div>
    <div v-loading="fileEditor.loading" class="file-editor-body">
      <CodeEditor
        v-if="editorReady"
        v-model="fileEditor.text"
        language="text"
        height="56vh"
        @save="saveTextFile()"
      />
    </div>
    <p v-if="fileEditor.msg" class="file-editor-msg muted">{{ fileEditor.msg }}</p>
    <template #footer>
      <span class="muted file-editor-hint">Ctrl / ⌘ + S 保存</span>
      <el-button plain @click="open = false">关闭</el-button>
      <el-button type="primary" :loading="fileEditor.loading" :disabled="busy || !fileEditor.path" @click="saveTextFile()">
        保存
      </el-button>
    </template>
  </el-dialog>
</template>
