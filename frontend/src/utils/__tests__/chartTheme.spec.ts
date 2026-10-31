import { describe, expect, it } from 'vitest'
import { efChartPalette, efLineStyle, withAlpha } from '../chartTheme'

describe('chart colors and line encoding', () => {
  it('expands short hex and returns parseable rgba rather than concatenated suffixes', () => {
    expect(withAlpha('#abc', 0.13)).toBe('rgba(170, 187, 204, 0.13)')
    expect(withAlpha(' #161616 ', 0.5)).toBe('rgba(22, 22, 22, 0.5)')
  })

  it('composes functional CSS colors without appending a hex suffix', () => {
    expect(withAlpha('rgba(22, 22, 22, 0.65)', 0.13))
      .toBe('color-mix(in srgb, rgba(22, 22, 22, 0.65) 13%, transparent)')
    expect(withAlpha('color-mix(in srgb, #161616 65%, transparent)', 0.13))
      .toBe('color-mix(in srgb, color-mix(in srgb, #161616 65%, transparent) 13%, transparent)')
  })

  it('clamps alpha and handles non-finite inputs', () => {
    expect(withAlpha('#000', -1)).toBe('rgba(0, 0, 0, 0)')
    expect(withAlpha('#000', 2)).toBe('rgba(0, 0, 0, 1)')
    expect(withAlpha('#000', NaN)).toBe('rgba(0, 0, 0, 0)')
  })

  it('encodes twelve series with categorical color plus unique line and point patterns', () => {
    const lines = Array.from({ length: 12 }, (_, i) => efLineStyle(i))
    expect(new Set(lines.map(line => JSON.stringify([line.borderDash, line.pointStyle]))).size).toBe(12)
    expect(new Set(lines.map(line => JSON.stringify(line.borderDash))).size).toBe(12)
    expect(lines.every(line => line.pointRadius >= 2 && !line.fill)).toBe(true)
    expect(lines[0].borderWidth).toBeGreaterThan(lines[1].borderWidth)
    expect(lines[0].borderColor).toBe(efChartPalette().steps[0])
    expect(new Set(lines.map(line => line.borderColor)).size).toBe(6)
    expect(lines.every(line => line.borderColor !== efChartPalette().ink)).toBe(true)
  })
})
