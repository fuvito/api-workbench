import { describe, it, expect } from 'vitest'
import { serializeScenarioToYAML } from './scenario-yaml'
import detail from '../../../testdata/scenario-roundtrip/detail.json?raw'
import saved from '../../../testdata/scenario-roundtrip/saved.yaml?raw'

// The other half of scenario_roundtrip_test.go, which holds that detail.json is what Go hands the
// editor and that saved.yaml is the same scenario as the file it came from.
describe('saving a loaded scenario', () => {
  it('writes the expected file', () => {
    expect(serializeScenarioToYAML(JSON.parse(detail))).toBe(saved.replace(/\r\n/g, '\n'))
  })
})
