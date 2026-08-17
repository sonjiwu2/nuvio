/**
 * A minimal squarified treemap layout (Bruls, Huizing, van Wijk).
 *
 * Deliberately hand-rolled instead of pulling in a charting library: the
 * input here is at most a couple dozen items (rootChildrenCap on the Go
 * side), and the full algorithm is well under a hundred lines.
 */

export interface TreemapItem {
  key: string
  label: string
  value: number
}

export interface TreemapRect extends TreemapItem {
  x: number
  y: number
  width: number
  height: number
}

interface Box {
  x: number
  y: number
  width: number
  height: number
}

/** Worst aspect ratio among a row of items laid out along a strip of length `side`. */
function worstRatio(row: number[], side: number): number {
  const sum = row.reduce((a, b) => a + b, 0)
  if (sum === 0 || side === 0) return Infinity

  const max = Math.max(...row)
  const min = Math.min(...row)
  const sideSq = side * side
  const sumSq = sum * sum

  return Math.max((sideSq * max) / sumSq, sumSq / (sideSq * min))
}

export function computeTreemap(items: TreemapItem[], width: number, height: number): TreemapRect[] {
  const positive = items.filter((item) => item.value > 0)
  if (positive.length === 0 || width <= 0 || height <= 0) return []

  const total = positive.reduce((sum, item) => sum + item.value, 0)
  const area = width * height
  const sorted = [...positive]
    .sort((a, b) => b.value - a.value)
    .map((item) => ({ ...item, area: (item.value / total) * area }))

  const rects: TreemapRect[] = []
  let remaining = sorted
  let box: Box = { x: 0, y: 0, width, height }

  while (remaining.length > 0) {
    const side = Math.min(box.width, box.height)
    let row: typeof sorted = []
    let rest = remaining

    while (rest.length > 0) {
      const candidateRow = [...row, rest[0]!]
      const candidateAreas = candidateRow.map((item) => item.area)
      if (
        row.length === 0 ||
        worstRatio(candidateAreas, side) <=
          worstRatio(
            row.map((r) => r.area),
            side,
          )
      ) {
        row = candidateRow
        rest = rest.slice(1)
      } else {
        break
      }
    }

    box = layoutRow(row, box, side, rects)
    remaining = rest
  }

  return rects
}

/** Places one row of items along the shorter side of `box`, returns the remaining box. */
function layoutRow(
  row: { key: string; label: string; value: number; area: number }[],
  box: Box,
  side: number,
  out: TreemapRect[],
): Box {
  const rowArea = row.reduce((sum, item) => sum + item.area, 0)
  const thickness = side === 0 ? 0 : rowArea / side

  const horizontal = box.width <= box.height
  let offset = 0

  for (const item of row) {
    const length = rowArea === 0 ? 0 : (item.area / rowArea) * side
    if (horizontal) {
      out.push({ ...item, x: box.x + offset, y: box.y, width: length, height: thickness })
    } else {
      out.push({ ...item, x: box.x, y: box.y + offset, width: thickness, height: length })
    }
    offset += length
  }

  if (horizontal) {
    return { x: box.x, y: box.y + thickness, width: box.width, height: box.height - thickness }
  }
  return { x: box.x + thickness, y: box.y, width: box.width - thickness, height: box.height }
}
