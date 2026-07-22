import assert from 'node:assert/strict'
import test from 'node:test'
import { ProcessGroup, runForeground, sleep } from './processes.mts'

test('runForeground returns the child process exit status', async () => {
	const result = await runForeground([process.execPath, '-e', 'process.exit(7)'])

	assert.equal(result.code, 7)
	assert.equal(result.signal, null)
})

test('ProcessGroup.shutdown stops managed child processes', async () => {
	const processes = new ProcessGroup()
	const managedProcess = processes.spawn(
		'test-child',
		[
			process.execPath,
			'-e',
			"process.on('SIGTERM', () => process.exit(0)); setInterval(() => undefined, 1000)",
		],
		{},
	)

	await processes.shutdown()

	const result = await Promise.race([
		managedProcess.exit,
		sleep(1000).then(() => {
			throw new Error('timed out waiting for managed process exit')
		}),
	])

	assert.equal(result.code, null)
	assert.equal(result.signal, 'SIGTERM')
})
