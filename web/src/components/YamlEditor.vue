<script setup>
/** site.yaml 高级编辑：CodeMirror YAML 高亮。 */
import CodeEditor from './CodeEditor.vue'

defineProps({
  modelValue: { type: String, default: '' },
})
defineEmits(['update:modelValue'])
</script>

<template>
  <CodeEditor
    :model-value="modelValue"
    language="yaml"
    height="420px"
    @update:model-value="$emit('update:modelValue', $event)"
  />
</template>
