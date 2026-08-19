import { useEffect, useRef } from 'react'

export function useSheet<T extends HTMLElement>(onClose: () => void, dismissable = true) {
  const container = useRef<T>(null)

  useEffect(() => {
    const restoreTo = document.activeElement as HTMLElement | null
    container.current?.focus({ preventScroll: true })

    return () => restoreTo?.focus?.({ preventScroll: true })
  }, [])

  useEffect(() => {
    if (!dismissable) {
      return
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        onClose()
      }
    }

    document.addEventListener('keydown', handleKeyDown)

    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [dismissable, onClose])

  return container
}
