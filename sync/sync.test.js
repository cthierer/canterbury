import assert from 'node:assert/strict'
import { mkdtemp, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { run } from './sync.js'

const captureOutput = () => {
	let value = ''
	return {
		output: { write: chunk => (value += chunk) },
		value: () => value,
	}
}

test('healthcheck reports only stable non-sensitive readiness status', async t => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-health-cli-'))
	const statePath = join(directory, 'health.json')
	t.mock.method(process, 'kill', () => true)
	t.mock.method(Date, 'now', () => 1_001)
	const previousStatePath = process.env.SYNC_HEALTH_STATE_PATH
	process.env.SYNC_HEALTH_STATE_PATH = statePath
	t.after(() => {
		if (previousStatePath === undefined) {
			delete process.env.SYNC_HEALTH_STATE_PATH
			return
		}
		process.env.SYNC_HEALTH_STATE_PATH = previousStatePath
	})
	await writeFile(
		statePath,
		`${JSON.stringify({
			schemaVersion: 1,
			workerPid: 101,
			childPid: 202,
			phase: 'continuous-sync',
			initialSyncComplete: true,
			updatedAt: 1_000,
			version: 'test',
			revision: 'abc123',
		})}\n`,
	)
	const capture = captureOutput()

	assert.equal(await run({ args: ['healthcheck'], output: capture.output }), 0)
	assert.equal(capture.value(), 'READY\n')
	assert.doesNotMatch(capture.value(), /vault|token|password/i)
})

test('healthcheck fails closed for invalid arguments', async () => {
	const capture = captureOutput()
	assert.equal(
		await run({ args: ['healthcheck', '--mode', 'dependency-details'], output: capture.output }),
		1,
	)
	assert.equal(capture.value(), 'NOT_READY\n')
})
