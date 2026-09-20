<script>
/** 本机路径选择器：目录、YAML、nacos*.zip、.sql 多选。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'PathPickerDialog',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <el-dialog
      v-model="picker.open"
      :title="picker.mode === 'yaml' ? '选择 YAML 文件' : picker.mode === 'nacos-zip' ? '选择 nacos*.zip（可多选）' : picker.mode === 'sql' ? '选择 .sql 文件（可多选）' : '选择目录'"
      width="640px"
      class="picker-dialog"
    >
      <p class="muted" style="margin:0 0 0.75rem">{{ picker.current || '选择盘符 / 根目录' }}</p>
      <div class="modal-toolbar" style="padding:0 0 0.75rem;border:0">
        <el-button type="primary" plain size="small" @click="browseFS(picker.parent)" :disabled="!picker.parent && picker.current">上级</el-button>
        <el-button type="primary" plain size="small" @click="browseFS('')">根 / 盘符</el-button>
        <el-button
          type="warning"
          plain
          size="small"
          v-if="picker.mode === 'dir' && picker.current"
          :loading="picker.expanding"
          @click="expandPickerDir"
        >{{ picker.expanding ? '解压中…' : '解压 ZIP / 展开 TAR' }}</el-button>
        <el-button
          type="primary"
          size="small"
          v-if="picker.mode === 'dir' && picker.current"
          @click="confirmPicker(picker.current)"
        >选择当前目录</el-button>
        <el-button
          type="primary"
          size="small"
          v-if="picker.mode === 'nacos-zip' || picker.mode === 'sql'"
          :disabled="!picker.selected.length"
          @click="picker.mode === 'sql' ? confirmSqlPicker() : confirmNacosZipPicker()"
        >确认已选 {{ picker.selected.length }} 个</el-button>
      </div>
      <p v-if="picker.mode === 'dir'" class="muted" style="font-size:0.85rem">
        目录内若有 <code>.zip</code> / <code>.tar.zip</code>，可先点「解压 ZIP / 展开 TAR」；含 <code>[manifest]</code> 用于部署，含 <code>[docker]</code> 用于 Docker 离线安装。
      </p>
      <p v-else-if="picker.mode === 'nacos-zip'" class="muted" style="font-size:0.85rem">
        仅显示文件名以 <code>nacos</code> 开头的 <code>.zip</code>；点击勾选，可多选后确认。导入时直接上传到 Nacos，不解压。
      </p>
      <p v-else-if="picker.mode === 'sql'" class="muted" style="font-size:0.85rem">
        仅显示 <code>.sql</code>；点击勾选，可多选后按选择顺序执行。文件在本机 Linux 磁盘上。
      </p>
      <div class="fs-list">
        <button
          v-for="e in picker.entries"
          :key="e.path"
          type="button"
          class="fs-item"
          :class="{ selected: (picker.mode === 'nacos-zip' || picker.mode === 'sql') && !e.isDir && isPickerSelected(e.path) }"
          @click="onFsClick(e)"
          @dblclick="onFsDblClick(e)"
        >
          <el-icon class="fs-icon"><Folder v-if="e.isDir" /><Document v-else /></el-icon>
          <span>{{ e.name }}</span>
          <el-icon v-if="(picker.mode === 'nacos-zip' || picker.mode === 'sql') && !e.isDir && isPickerSelected(e.path)"><Check /></el-icon>
        </button>
        <el-empty v-if="!(picker.entries && picker.entries.length)" description="空目录或无可选文件" :image-size="72" />
      </div>
      <el-alert v-if="picker.expandMsg" type="success" :closable="false" :title="picker.expandMsg" style="margin-top:0.5rem" />
      <el-alert v-if="picker.error" type="error" :closable="false" :title="picker.error" style="margin-top:0.5rem" />
    </el-dialog>
</template>
