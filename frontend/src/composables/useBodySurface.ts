import { onBeforeUnmount, onMounted, toValue, watch, type MaybeRefOrGetter } from 'vue'

// Teleported controls need the page's styling context without a body :has()
// selector, which invalidates styles across the document when content changes.
const owners = new WeakMap<HTMLElement, Map<string, number>>()

export function useBodySurface(classes: string[], enabled: MaybeRefOrGetter<boolean> = true) {
  let release: (() => void) | undefined
  let stop: (() => void) | undefined
  onMounted(() => {
    const body = document.body
    const counts = owners.get(body) ?? new Map<string, number>()
    owners.set(body, counts)
    stop = watch(() => toValue(enabled), active => {
      release?.()
      release = undefined
      if (!active) return
      for (const name of classes) {
        counts.set(name, (counts.get(name) ?? 0) + 1)
        body.classList.add(name)
      }
      release = () => {
        for (const name of classes) {
          const count = (counts.get(name) ?? 1) - 1
          if (count > 0) counts.set(name, count)
          else { counts.delete(name); body.classList.remove(name) }
        }
      }
    }, { immediate: true, flush: 'sync' })
  })
  onBeforeUnmount(() => { stop?.(); release?.() })
}
