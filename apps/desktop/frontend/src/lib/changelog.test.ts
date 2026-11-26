import { describe, it, expect } from 'vitest'
import { parseChangelog } from './changelog'
import changelog from '../../../wails/help/changelog.md?raw'

describe('parseChangelog', () => {
  it('reads headings and their lines, whatever the line endings, and ignores the rest', () => {
    const text = '# Changelog\r\n\r\n- not in a release\r\n## 1.2.0 — 2026-01-02\r\n\r\n- one\r\nprose\r\n- two\r\n## Notes\r\n## 1.1.0 — 2026-01-01\r\n- three\r\n'
    expect(parseChangelog(text)).toEqual([
      { version: '1.2.0', date: '2026-01-02', items: ['one', 'two'] },
      { version: '1.1.0', date: '2026-01-01', items: ['three'] },
    ])
  })

  it('reads the shipped file: every release has changes, newest first, none twice', () => {
    const releases = parseChangelog(changelog)
    expect(releases.length).toBeGreaterThanOrEqual(35)
    expect(releases.every(r => r.items.length > 0)).toBe(true)
    // every "## " heading of the file was understood
    expect(releases.length).toBe(changelog.split('\n').filter(l => l.startsWith('## ')).length)
    const order = releases.map(r => r.version.split('.').map(Number).reduce((n, part) => n * 1000 + part, 0))
    expect(order).toEqual([...new Set(order)].sort((a, b) => b - a))
  })
})
