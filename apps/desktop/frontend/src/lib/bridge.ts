// Typed wrapper around Wails-generated bindings, in parts by topic under bridge/.
// Uses plain interfaces for ergonomic object creation at call sites.
// The generated wailsjs classes use 'namespace' (not erasable syntax in TS7),
// so all Wails imports are isolated in these files. Import from '@/lib/bridge'.
export * from './bridge/core'
export * from './bridge/workspace'
export * from './bridge/imports'
export * from './bridge/cli'
export * from './bridge/conformance'
export * from './bridge/scenarios'
export * from './bridge/help'
