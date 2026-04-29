// M4.2 — SVG overlay that draws bezier connection lines between cards
// sharing the same focus_key across the three dossier columns.
//
// Strategy:
//   • Cards expose their focus_key via a `data-focus-key` HTML attribute.
//   • This component queries those elements within the panel container,
//     computes their positions relative to the container, and draws
//     cubic bezier curves connecting them left→middle→right.
//   • A capturing scroll listener + ResizeObserver keep lines in sync when
//     any column is scrolled or the window resizes.
//
// Line routing:
//   EB (right-edge midpoint) → VB (left-edge midpoint)  — per matching pair
//   VB (right-edge midpoint) → RF (left-edge midpoint)  — per matching pair
//   EB → RF directly when no VB card exists for the key (bridge gap indicator)
import { useEffect, useRef, useState, type RefObject } from 'react'

const LINE_COLOR = '#60a5fa'  // tailwind blue-400, matches ring-blue-400
const LINE_WIDTH = 1.5
const LINE_DASH  = '6 3'
const LINE_OPACITY = 0.75

interface Point { x: number; y: number }

function bezierPath(from: Point, to: Point): string {
  const cx = (from.x + to.x) / 2
  return `M ${from.x} ${from.y} C ${cx} ${from.y}, ${cx} ${to.y}, ${to.x} ${to.y}`
}

/** Midpoint of the right edge of `el` relative to `cRect`. */
function midRight(el: Element, cRect: DOMRect): Point {
  const r = el.getBoundingClientRect()
  return { x: r.right - cRect.left, y: (r.top + r.bottom) / 2 - cRect.top }
}

/** Midpoint of the left edge of `el` relative to `cRect`. */
function midLeft(el: Element, cRect: DOMRect): Point {
  const r = el.getBoundingClientRect()
  return { x: r.left - cRect.left, y: (r.top + r.bottom) / 2 - cRect.top }
}

interface FocusLinkOverlayProps {
  activeFocusKey: string | null
  /**
   * Ref to the flex container that directly holds the three column wrapper
   * divs (left / middle / right). Must have `position: relative` so the
   * absolutely-positioned SVG covers it correctly.
   */
  panelRef: RefObject<HTMLDivElement>
}

export function FocusLinkOverlay({ activeFocusKey, panelRef }: FocusLinkOverlayProps) {
  const [paths, setPaths] = useState<string[]>([])
  const rafRef = useRef<number | null>(null)

  useEffect(() => {
    const container = panelRef.current
    if (!activeFocusKey || !container) {
      setPaths([])
      return
    }

    const escaped = CSS.escape(activeFocusKey)
    const sel = `[data-focus-key="${escaped}"]`

    function compute() {
      if (!container) return
      const cRect = container.getBoundingClientRect()
      // The three direct children are the column wrapper divs
      const cols = Array.from(container.children) as Element[]
      if (cols.length < 3) return

      const ebCards = Array.from(cols[0].querySelectorAll(sel))
      const vbCards = Array.from(cols[1].querySelectorAll(sel))
      const rfCards = Array.from(cols[2].querySelectorAll(sel))

      const newPaths: string[] = []

      // EB → VB
      for (const eb of ebCards) {
        for (const vb of vbCards) {
          newPaths.push(bezierPath(midRight(eb, cRect), midLeft(vb, cRect)))
        }
      }

      // VB → RF
      for (const vb of vbCards) {
        for (const rf of rfCards) {
          newPaths.push(bezierPath(midRight(vb, cRect), midLeft(rf, cRect)))
        }
      }

      // EB → RF (direct — causal bridge has no matching entry for this key)
      if (vbCards.length === 0) {
        for (const eb of ebCards) {
          for (const rf of rfCards) {
            newPaths.push(bezierPath(midRight(eb, cRect), midLeft(rf, cRect)))
          }
        }
      }

      setPaths(newPaths)
    }

    function scheduleCompute() {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current)
      rafRef.current = requestAnimationFrame(compute)
    }

    // Initial render
    compute()

    // Capture scroll events from any column's overflow-y-auto pane
    container.addEventListener('scroll', scheduleCompute, { capture: true, passive: true })

    // Column width / card height / viewport size changes
    const ro = new ResizeObserver(scheduleCompute)
    ro.observe(container)

    return () => {
      container.removeEventListener('scroll', scheduleCompute, { capture: true })
      ro.disconnect()
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current)
    }
  }, [activeFocusKey, panelRef])

  if (!activeFocusKey || paths.length === 0) return null

  return (
    <svg
      aria-hidden="true"
      className="absolute inset-0 w-full h-full pointer-events-none"
    >
      {paths.map((d, i) => (
        <path
          key={i}
          d={d}
          fill="none"
          stroke={LINE_COLOR}
          strokeWidth={LINE_WIDTH}
          strokeDasharray={LINE_DASH}
          opacity={LINE_OPACITY}
        />
      ))}
    </svg>
  )
}
