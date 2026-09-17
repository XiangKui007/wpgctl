<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
import { yaml } from '@codemirror/lang-yaml'

const props = defineProps({
  modelValue: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const host = ref(null)
let view = null
let applying = false

const theme = EditorView.theme({
  '&': {
    height: '420px',
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

onMounted(() => {
  if (!host.value) return
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
      yaml(),
      theme,
      keymap.of([indentWithTab, ...defaultKeymap, ...historyKeymap]),
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
  <div ref="host" class="yaml-cm" />
</template>

<style scoped>
.yaml-cm {
  width: 100%;
  border-radius: 4px;
  overflow: hidden;
}
</style>
