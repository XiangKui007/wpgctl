<script>
/**
 * 本机路径选择器：目录、YAML、nacos*.zip、.sql 多选。
 * 选目录时同时展示文件夹下的文件，并用一行短提示说明当前能不能选。
 * 解压走「解压当前目录」或条目上的「解压此包」，并显示进度。
 */
import { nextTick, ref, watch } from 'vue'
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'PathPickerDialog',
  setup() {
    const ctx = useConsole()
    const expandLogEl = ref(null)
    watch(
      () => ctx.picker.expandLogs.length,
      async () => {
        await nextTick()
        const el = expandLogEl.value
        if (el) el.scrollTop = el.scrollHeight
      },
    )
    return { ...ctx, expandLogEl }
  },
}
</script>

<template>
    <el-dialog
      v-model="picker.open"
      :title="picker.mode === 'yaml' ? '选择 YAML 文件' : picker.mode === 'nacos-zip' ? '选择 nacos*.zip（可多选）' : picker.mode === 'sql' ? '选择 .sql 文件（可多选）' : (picker.hint && picker.hint.title ? '选择' + picker.hint.title : '选择目录')"
      width="720px"
      :class="['picker-dialog', { 'is-expanding': picker.expanding || picker.expandLogs.length }]"
      :close-on-click-modal="!picker.expanding"
      :close-on-press-escape="!picker.expanding"
    >
      <p class="picker-current">
        <span class="muted">当前目录</span>
        <code>{{ picker.current || '选择盘符 / 根目录' }}</code>
      </p>
      <div class="modal-toolbar" style="padding:0 0 0.75rem;border:0">
        <el-button type="primary" plain size="small" @click="browseFS(picker.parent)" :disabled="picker.expanding || (!picker.parent && picker.current)">上级</el-button>
        <el-button type="primary" plain size="small" @click="browseFS('')" :disabled="picker.expanding">根 / 盘符</el-button>
        <el-button type="primary" plain size="small" @click="browseFS(picker.current)" :disabled="picker.expanding">
          <el-icon style="margin-right:0.25em"><Refresh /></el-icon>刷新
        </el-button>
        <el-button
          type="warning"
          plain
          size="small"
          v-if="picker.mode === 'dir' && picker.current"
          :loading="picker.expanding"
          :disabled="picker.expanding"
          @click="expandPickerDir"
        >{{ picker.expanding ? '解压中…' : '解压当前目录' }}</el-button>
        <el-button
          type="primary"
          size="small"
          v-if="picker.mode === 'dir' && picker.current"
          :class="{ 'is-ready-pick': picker.hint && picker.hint.level === 'ready' }"
          :disabled="picker.expanding"
          @click="confirmPicker(picker.current)"
        >{{ picker.hint && picker.hint.level === 'ready' ? '就选这一层' : '选择当前目录' }}</el-button>
        <el-button
          type="primary"
          size="small"
          v-if="picker.mode === 'nacos-zip' || picker.mode === 'sql'"
          :disabled="!picker.selected.length || picker.expanding"
          @click="picker.mode === 'sql' ? confirmSqlPicker() : confirmNacosZipPicker()"
        >确认已选 {{ picker.selected.length }} 个</el-button>
      </div>
      <p
        v-if="picker.mode === 'dir' && picker.hint && picker.hint.headline"
        class="picker-level-hint"
        :data-level="picker.hint.level"
      >
        {{ picker.hint.headline }}<template v-if="picker.hint.level !== 'ready' && picker.hint.message"> · {{ picker.hint.message }}</template>
      </p>
      <p v-if="picker.mode === 'nacos-zip'" class="muted" style="font-size:0.85rem">
        勾选 nacos*.zip，导入时不解压。
      </p>
      <p v-else-if="picker.mode === 'sql'" class="muted" style="font-size:0.85rem">
        勾选 .sql，按选择顺序执行。
      </p>
      <div v-if="picker.expanding || picker.expandLogs.length" class="picker-expand-progress" :class="{ live: picker.expanding }">
        <div class="picker-expand-head">
          <p class="picker-expand-status">{{ picker.expandStatus }}</p>
          <strong class="picker-expand-pct">{{ picker.expandPercent }}%</strong>
        </div>
        <p v-if="picker.expandPackTotal > 1" class="picker-expand-label">总体</p>
        <el-progress
          :percentage="picker.expandPercent"
          :status="picker.expanding ? undefined : (picker.error ? 'exception' : 'success')"
          :stroke-width="16"
          striped
          :striped-flow="picker.expanding"
          :color="picker.expanding ? '#12b5a2' : undefined"
        />
        <template v-if="picker.expandPackTotal > 1">
          <div class="picker-expand-pack-head">
            <p class="picker-expand-label">当前包 {{ picker.expandPackPercent }}%</p>
          </div>
          <el-progress
            :percentage="picker.expandPackPercent"
            :stroke-width="10"
            striped
            :striped-flow="picker.expanding"
            color="#12b5a2"
          />
        </template>
        <p v-if="picker.expandDetail" class="picker-expand-detail">{{ picker.expandDetail }}</p>
        <pre ref="expandLogEl" class="log-box picker-expand-log">{{ picker.expandLogs.join('\n') }}</pre>
      </div>
      <div class="fs-list" :class="{ disabled: picker.expanding }">
        <button
          v-for="e in picker.entries"
          :key="e.path"
          type="button"
          class="fs-item"
          :class="{
            selected: (picker.mode === 'nacos-zip' || picker.mode === 'sql') && !e.isDir && isPickerSelected(e.path),
            archive: e.isArchive,
            file: picker.mode === 'dir' && !e.isDir && !e.isArchive,
            'hint-enter': isHintEnter(e),
            'hint-mark': isHintMark(e),
          }"
          :disabled="picker.expanding"
          @click="onFsClick(e)"
          @dblclick="onFsDblClick(e)"
        >
          <el-icon class="fs-icon"><Folder v-if="e.isDir" /><Files v-else-if="e.isArchive" /><Document v-else /></el-icon>
          <span class="fs-name">{{ e.name }}</span>
          <span v-if="e.isArchive" class="fs-kind">{{ e.alreadyExpanded ? '已展开' : (e.archiveKind || 'zip') }}</span>
          <span v-else-if="isHintEnter(e)" class="fs-mark-icon enter" title="再进这一层">
            <el-icon><ArrowRight /></el-icon>
          </span>
          <span v-else-if="isHintMark(e)" class="fs-mark-icon" title="这一层的标志">
            <el-icon><CircleCheck /></el-icon>
          </span>
          <span v-else-if="picker.mode === 'dir' && !e.isDir" class="fs-kind">文件</span>
          <el-button
            v-if="picker.mode === 'dir' && canExpandArchive(e)"
            type="warning"
            plain
            size="small"
            :disabled="picker.expanding"
            @click.stop="expandPickerArchive(e)"
          >解压此包</el-button>
          <el-icon v-if="(picker.mode === 'nacos-zip' || picker.mode === 'sql') && !e.isDir && isPickerSelected(e.path)"><Check /></el-icon>
        </button>
        <el-empty v-if="!(picker.entries && picker.entries.length)" description="空目录或无可选文件" :image-size="72" />
      </div>
      <el-alert v-if="picker.expandHint && !picker.expanding" type="info" :closable="false" :title="picker.expandHint" style="margin-top:0.5rem" />
      <el-alert v-if="picker.expandMsg" type="success" :closable="false" :title="picker.expandMsg" style="margin-top:0.5rem" />
      <el-alert v-if="picker.error" type="error" :closable="false" :title="picker.error" style="margin-top:0.5rem" />
    </el-dialog>
</template>
