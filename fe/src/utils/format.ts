const formatter = new Intl.NumberFormat('en-US', {
  notation: 'compact',
  maximumFractionDigits: 1,
})

export function formatNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return '0'
  }
  return formatter.format(value)
}
