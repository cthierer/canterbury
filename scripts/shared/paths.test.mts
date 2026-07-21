import { basename, join } from 'node:path'
import assert from 'node:assert/strict'
import test from 'node:test'
import { brunoExe, brunoTestDir, localPomeriumDir, repoRoot } from './paths.mts'

test('repoRoot resolves to the repository root', () => {
	assert.equal(basename(repoRoot()), 'canterbury')
})

test('brunoTestDir resolves a named Bruno collection directory', () => {
	assert.equal(brunoTestDir('local-auth-smoke'), join(repoRoot(), 'bruno', 'local-auth-smoke'))
})

test('localPomeriumDir resolves the local Pomerium deployment directory', () => {
	assert.equal(localPomeriumDir(), join(repoRoot(), 'deploy', 'local-pomerium'))
})

test('brunoExe resolves the platform-specific Bruno executable', () => {
	assert.equal(brunoExe({ platform: 'linux' }), join(repoRoot(), 'node_modules', '.bin', 'bru'))
	assert.equal(brunoExe({ platform: 'win32' }), join(repoRoot(), 'node_modules', '.bin', 'bru.cmd'))
})
