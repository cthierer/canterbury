import assert from 'node:assert/strict'
import { mkdtemp, readFile, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import {
	DEFAULT_HEARTBEAT_MAX_AGE_MS,
	HealthReporter,
	evaluateHealth,
	readHealth,
} from './health.js'

const readyState = {
	schemaVersion: 1,
	workerPid: 101,
	childPid: 202,
	phase: 'continuous-sync',
	initialSyncComplete: true,
	updatedAt: 1_000,
	version: 'test',
	revision: 'abc123',
}

test('evaluateHealth distinguishes liveness from readiness', () => {
	const initializing = {
		...readyState,
		childPid: null,
		phase: 'initializing',
		initialSyncComplete: false,
	}
	const isAlive = () => true

	assert.equal(evaluateHealth({ state: initializing, mode: 'live', now: 1_001, isAlive }), true)
	assert.equal(evaluateHealth({ state: initializing, mode: 'ready', now: 1_001, isAlive }), false)
	assert.equal(evaluateHealth({ state: readyState, mode: 'ready', now: 1_001, isAlive }), true)
})

test('evaluateHealth fails closed for stale, future, stopped, and dead process states', () => {
	const isAlive = pid => pid !== readyState.childPid

	assert.equal(
		evaluateHealth({
			state: readyState,
			mode: 'live',
			now: readyState.updatedAt + DEFAULT_HEARTBEAT_MAX_AGE_MS + 1,
			isAlive: () => true,
		}),
		false,
	)
	assert.equal(
		evaluateHealth({
			state: readyState,
			mode: 'live',
			now: readyState.updatedAt - 1,
			isAlive: () => true,
		}),
		false,
	)
	assert.equal(
		evaluateHealth({
			state: { ...readyState, phase: 'shutting-down' },
			mode: 'live',
			now: 1_001,
			isAlive: () => true,
		}),
		false,
	)
	assert.equal(evaluateHealth({ state: readyState, mode: 'ready', now: 1_001, isAlive }), false)
})

test('readHealth rejects missing and malformed state', async () => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-health-read-'))
	const statePath = join(directory, 'health.json')

	assert.equal(await readHealth({ statePath, mode: 'ready' }), false)
	await writeFile(statePath, '{not-json}\n')
	assert.equal(await readHealth({ statePath, mode: 'ready' }), false)
})

test('HealthReporter writes atomic private state and transitions to ready', async () => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-health-report-'))
	const statePath = join(directory, 'health.json')
	let now = 1_000
	const reporter = new HealthReporter({
		statePath,
		workerPid: 101,
		version: '1.2.3',
		revision: 'abc123',
		now: () => now,
		heartbeatIntervalMs: 60_000,
	})

	await reporter.start()
	await reporter.update({ childPid: 202, phase: 'continuous-sync', initialSyncComplete: true })

	const state = JSON.parse(await readFile(statePath, 'utf8'))
	assert.deepEqual(state, {
		...readyState,
		version: '1.2.3',
		revision: 'abc123',
	})
	assert.equal((await stat(statePath)).mode & 0o777, 0o600)
	assert.equal(
		await readHealth({ statePath, mode: 'ready', now: now + 1, isAlive: () => true }),
		true,
	)

	now += 10
	await reporter.update({ phase: 'shutting-down' })
	assert.equal(
		await readHealth({ statePath, mode: 'live', now: now + 1, isAlive: () => true }),
		false,
	)
	await reporter.stop()
})
