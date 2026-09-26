// 按模块拆分的文案文件（同目录下除 index.ts 外的每个 .ts 默认导出一个对象），
// 以文件名作为命名空间合并，如 dashboard.ts → t('dashboard.xxx')
const modules = import.meta.glob<Record<string, unknown>>(['./*.ts', '!./index.ts'], { eager: true, import: 'default' })

const messages: Record<string, unknown> = {}
for (const [path, mod] of Object.entries(modules)) {
  messages[path.slice(2, -3)] = mod
}

export default messages
