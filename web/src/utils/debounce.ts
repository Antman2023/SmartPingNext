/**
 * 防抖函数
 * @param fn 要执行的函数
 * @param delay 延迟时间（毫秒）
 */
export function debounce<T extends (...args: unknown[]) => unknown>(
  fn: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timer: number | null = null

  return function (this: unknown, ...args: Parameters<T>) {
    if (timer !== null) {
      window.clearTimeout(timer)
    }
    timer = window.setTimeout(() => {
      fn.apply(this, args)
      timer = null
    }, delay)
  }
}

/**
 * 节流函数
 * @param fn 要执行的函数
 * @param delay 间隔时间（毫秒）
 */
export function throttle<T extends (...args: unknown[]) => unknown>(
  fn: T,
  delay: number
): (...args: Parameters<T>) => void {
  let lastTime = 0

  return function (this: unknown, ...args: Parameters<T>) {
    const now = Date.now()
    if (now - lastTime >= delay) {
      fn.apply(this, args)
      lastTime = now
    }
  }
}
