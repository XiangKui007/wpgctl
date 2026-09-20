<script setup>
/**
 * 等宽文本编辑器：site.yaml、.env、nginx conf 共用 CodeMirror。
 * @param {string} modelValue 文件全文
 * @param {'yaml'|'text'} language yaml 高亮；.env / conf 用纯文本即可
 * @param {string} height 编辑区高度
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
import { yaml } from '@codemirror/lang-yaml'

const props = defineProps({
  modelValue: { type: String, default: '' },
  language: { type: String, default: 'text' },
  height: { type: String, default: '420px' },
})
const emit = defineEmits(['update:modelValue', 'save'])

const host = ref(null)
let view = null
let applying = false

function buildTheme() {
  return EditorView.theme({
    '&': {
      height: props.height,
      fontSize: '13.5px',
      border: '1px solid rgba(21, 36, 40, 0.08)',
      borderRadius: '6px',
      backgroundColor: '#fbfcfc',
    },
    '.cm-scroller': {
      fontFamily: 'ui-monospace, "Cascadia Code", Consolas, monospace',
      lineHeight: '1.55',
    },
    '.cm-content': {
      caretColor: '#0f766e',
      padding: '8px 0',
    },
    '.cm-gutters': {
      backgroundColor: '#f3f7f6',
      color: '#7a9098',
      borderRight: '1px solid rgba(21, 36, 40, 0.06)',
    },
    '.cm-activeLine': {
      backgroundColor: 'rgba(20, 184, 166, 0.06)',
    },
    '.cm-activeLineGutter': {
      backgroundColor: 'rgba(20, 184, 166, 0.1)',
    },
    '&.cm-focused': {
      outline: '2px solid rgba(15, 118, 110, 0.35)',
      outlineOffset: '-1px',
    },
    '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
      backgroundColor: 'rgba(15, 118, 110, 0.18) !important',
    },
  })
}

onMounted(() => {
  if (!host.value) return
  const lang = props.language === 'yaml' ? [yaml()] : []
  const state = EditorState.create({
    doc: props.modelValue || '',
    extensions: [
      lineNumbers(),
      highlightActiveLine(),
      highlightActiveLineGutter(),
      foldGutter(),
      history(),
      indentOnInput(),
      bracketMatching(),
      ...lang,
      buildTheme(),
      keymap.of([
        indentWithTab,
        ...defaultKeymap,
        ...historyKeymap,
        {
          key: 'Mod-s',
          preventDefault: true,
          run: () => {
            emit('save')
            return true
          },
        },
      ]),
      EditorView.updateListener.of((update) => {
        if (!update.docChanged || applying) return
        emit('update:modelValue', update.state.doc.toString())
      }),
      EditorView.lineWrapping,
    ],
  })
  view = new EditorView({ state, parent: host.value })
})

watch(
  () => props.modelValue,
  (val) => {
    if (!view) return
    const cur = view.state.doc.toString()
    if (val === cur) return
    applying = true
    view.dispatch({
      changes: { from: 0, to: cur.length, insert: val || '' },
    })
    applying = false
  },
)

onBeforeUnmount(() => {
  if (view) {
    view.destroy()
    view = null
  }
})
</script>

<template>
  <div ref="host" class="code-cm" />
</template>

<style scoped>
.code-cm {
  width: 100%;
  border-radius: 4px;
  overflow: hidden;
}
</style>
