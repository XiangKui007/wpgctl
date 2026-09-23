<script>
/**
 * 任务日志框：向导各步 / 包中心 / 升级页共用。
 * 新日志到达时自动滚到底；用户手动往上翻则停住，不抢滚动条。
 */
import { nextTick, ref, watch } from 'vue'
import { logClass } from '@/utils/format.js'

export default {
  name: 'JobLogBox',
  props: {
    /** 日志行数组；为空时整个框不渲染。 */
    lines: { type: Array, default: () => [] },
    /** 任务是否仍在跑；为真时右上角显示转圈提示。 */
    running: { type: Boolean, default: false },
  },
  setup(props) {
    const box = ref(null)
    const pinBottom = ref(true)
    function onScroll() {
      const el = box.value
      if (!el) return
      pinBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48
    }
    watch(
      () => props.lines.length,
      async (n, prev) => {
        // 新任务清空日志时重新贴底。
        if (n < (prev || 0)) pinBottom.value = true
        if (!pinBottom.value) return
        await nextTick()
        const el = box.value
        if (el) el.scrollTop = el.scrollHeight
      },
    )
    return { box, onScroll, logClass }
  },
}
</script>

<template>
  <div v-if="lines.length" ref="box" class="log-box job-log-box" :class="{ live: running }" @scroll="onScroll">
    <div v-if="running" class="job-log-running">
      <el-icon class="is-loading"><Loading /></el-icon>
      执行中
    </div>
    <div v-for="(l, i) in lines" :key="i" :class="logClass(l)">{{ l }}</div>
  </div>
</template>
