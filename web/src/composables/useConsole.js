/**
 * 读取现场控制台根状态。
 * 仅在 App.vue provide(createConsole()) 之后的子组件里调用。
 *
 * @returns {object} createConsole() 交给模板的绑定（ref 保持为 ref）
 */
import { inject } from 'vue'
import { WPGCTL_KEY } from './key.js'

export function useConsole() {
  const ctx = inject(WPGCTL_KEY)
  if (!ctx) {
    throw new Error('useConsole() 只能在控制台根组件的子树中使用')
  }
  return ctx
}
