import type { ScenarioDetail } from '@/lib/bridge'

function yamlScalar(s: string): string {
  if (s === '') return '""'
  if (
    /: |[#[\]{},&*!|>'"%@`\n\r\t]/.test(s) ||
    s.startsWith(' ') || s.endsWith(' ') ||
    s === 'true' || s === 'false' || s === 'null'
  ) return JSON.stringify(s)
  return s
}

function yamlValue(v: unknown): string {
  if (v === null || v === undefined) return 'null'
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  // a list or an object, written as JSON, which YAML reads as the same value
  if (typeof v === 'object') return JSON.stringify(v)
  return yamlScalar(String(v))
}

/**
 * Writes an item's x- keys back under it. Each line is `"x-key": <JSON>`, made by Go from the file,
 * and is valid YAML as it stands.
 */
export function pushExtensions(lines: string[], indent: string, extensions: string | undefined): void {
  for (const line of extensions?.split('\n') ?? []) lines.push(indent + line)
}

export function serializeScenarioToYAML(d: ScenarioDetail): string {
  const lines: string[] = ['schemaVersion: "1"', `title: ${yamlScalar(d.title)}`]
  if (d.description) lines.push(`description: ${yamlScalar(d.description)}`)
  if (d.specFile) lines.push(`specFile: ${yamlScalar(d.specFile)}`)
  if (d.variables?.length) {
    lines.push('variables:')
    for (const v of d.variables) {
      lines.push(`  - name: ${yamlScalar(v.name)}`)
      lines.push(`    value: ${yamlScalar(v.value)}`)
      if (v.secret) lines.push('    secret: true')
      pushExtensions(lines, '    ', v.extensions)
    }
  }
  pushExtensions(lines, '', d.extensions)
  if (!d.steps.length) {
    lines.push('steps: []')
  } else {
    lines.push('steps:')
    for (const step of d.steps) {
      lines.push(`  - label: ${yamlScalar(step.label)}`)
      if (step.notes) lines.push(`    notes: ${yamlScalar(step.notes)}`)
      if (step.disabled) lines.push('    disabled: true')
      if (step.requestRef) {
        lines.push(`    requestRef: ${yamlScalar(step.requestRef)}`)
      } else if (step.method || step.url) {
        lines.push('    request:')
        if (step.requestTitle) lines.push(`      title: ${yamlScalar(step.requestTitle)}`)
        if (step.method) lines.push(`      method: ${step.method}`)
        if (step.url) lines.push(`      url: ${yamlScalar(step.url)}`)
        if (step.noFollowRedirects) lines.push('      followRedirects: false')
        if (step.requestTimeout) lines.push(`      timeout: ${yamlScalar(step.requestTimeout)}`)
        pushExtensions(lines, '      ', step.requestExtensions)
        if (step.headers?.length) {
          lines.push('      headers:')
          for (const h of step.headers) {
            lines.push(`        - name: ${yamlScalar(h.name)}`)
            lines.push(`          value: ${yamlScalar(h.value)}`)
            pushExtensions(lines, '          ', h.extensions)
          }
        }
        if (step.queryParams?.length) {
          lines.push('      queryParams:')
          for (const q of step.queryParams) {
            lines.push(`        - name: ${yamlScalar(q.name)}`)
            lines.push(`          value: ${yamlScalar(q.value)}`)
            pushExtensions(lines, '          ', q.extensions)
          }
        }
        if (step.bodyType && step.bodyType !== 'none') {
          lines.push('      body:')
          lines.push(`        type: ${step.bodyType}`)
          if (step.bodyContent) lines.push(`        content: ${yamlScalar(step.bodyContent)}`)
          pushExtensions(lines, '        ', step.bodyExtensions)
        }
        const a = step.auth
        if (a && (a.type === 'bearer' || a.type === 'basic' || a.type === 'apikey')) {
          lines.push('      auth:')
          lines.push(`        type: ${a.type}`)
          if (a.type === 'bearer') lines.push(`        token: ${yamlScalar(a.token ?? '')}`)
          if (a.type === 'basic') {
            lines.push(`        username: ${yamlScalar(a.username ?? '')}`)
            lines.push(`        password: ${yamlScalar(a.password ?? '')}`)
          }
          if (a.type === 'apikey') {
            lines.push(`        key: ${yamlScalar(a.key ?? '')}`)
            lines.push(`        value: ${yamlScalar(a.value ?? '')}`)
            lines.push(`        location: ${a.location || 'header'}`)
          }
          pushExtensions(lines, '        ', a.extensions)
        }
      }
      if (step.failPolicy && step.failPolicy !== 'stop') {
        lines.push(`    failPolicy: ${step.failPolicy}`)
      }
      if (step.timeoutMs && step.timeoutMs > 0) {
        lines.push(`    timeoutMs: ${step.timeoutMs}`)
      }
      if (step.retryMaxAttempts && step.retryMaxAttempts > 0) {
        lines.push('    retry:')
        lines.push(`      maxAttempts: ${step.retryMaxAttempts}`)
        if (step.retryDelayMs && step.retryDelayMs > 0) {
          lines.push(`      delayMs: ${step.retryDelayMs}`)
        }
        pushExtensions(lines, '      ', step.retryExtensions)
      }
      if (step.assertions?.length) {
        lines.push('    assertions:')
        for (const a of step.assertions) {
          lines.push(`      - type: ${a.type}`)
          if (a.name) lines.push(`        name: ${yamlScalar(a.name)}`)
          if (a.path) lines.push(`        path: ${yamlScalar(a.path)}`)
          lines.push(`        operator: ${a.operator ?? 'eq'}`)
          if (a.expected !== undefined && a.expected !== null) {
            lines.push(`        expected: ${yamlValue(a.expected)}`)
          }
          if (a.schema) lines.push(`        schema: ${yamlScalar(a.schema)}`)
          pushExtensions(lines, '        ', a.extensions)
        }
      }
      if (step.extractions?.length) {
        lines.push('    extractions:')
        for (const e of step.extractions) {
          lines.push(`      - name: ${yamlScalar(e.name)}`)
          lines.push(`        source: ${e.source}`)
          if (e.path) lines.push(`        path: ${yamlScalar(e.path)}`)
          pushExtensions(lines, '        ', e.extensions)
        }
      }
      pushExtensions(lines, '    ', step.extensions)
    }
  }
  return lines.join('\n') + '\n'
}
