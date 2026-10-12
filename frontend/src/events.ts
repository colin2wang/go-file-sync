// 事件封装：桌面端走 Wails runtime，浏览器端走内存 emitter（Mock）
type Handler = (data: any) => void

const handlers = new Map<string, Set<Handler>>()

function isWails(): boolean {
  return typeof (window as any).runtime !== 'undefined' && typeof (window as any).runtime.EventsOn === 'function'
}

export function onEvent(name: string, cb: Handler): () => void {
  if (isWails()) {
    return (window as any).runtime.EventsOn(name, cb)
  }
  let set = handlers.get(name)
  if (!set) {
    set = new Set()
    handlers.set(name, set)
  }
  set.add(cb)
  return () => set!.delete(cb)
}

// 仅 Mock 后端使用
export function emitMock(name: string, data?: any): void {
  const set = handlers.get(name)
  if (set) set.forEach((cb) => cb(data))
}
