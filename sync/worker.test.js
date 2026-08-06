import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import { mkdtemp, readFile, stat } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { readHealth } from './health.js'
import {
	ProcessSupervisor,
	describeCommand,
	loadWorkerConfig,
	runWorker,
	validateEnvironment,
} from './worker.js'

const waitFor = async predicate => {
	for (let attempt = 0; attempt < 100; attempt += 1) {
		if (await predicate()) {
			return
		}
		await new Promise(resolve => setTimeout(resolve, 5))
	}
	throw new Error('Timed out waiting for condition')
}

const createChild = pid => {
	const child = new EventEmitter()
	child.pid = pid
	child.signals = []
	child.kill = signal => {
		child.signals.push(signal)
		return true
	}
	return child
}

const testConfig = directory => ({
	vaultPath: join(directory, 'vault'),
	vaultName: 'fake-vault',
	vaultPassword: 'fake-password',
	obsidianAuthToken: 'fake-token',
	deviceName: 'test-device',
	statePath: join(directory, 'run', 'health.json'),
	authTokenPath: join(directory, 'config', 'auth_token'),
	version: 'test',
	revision: 'abc123',
})

test('validateEnvironment uses distinct configuration exit codes', () => {
	for (const [property, exitCode] of [
		['vaultName', 10],
		['vaultPassword', 11],
		['obsidianAuthToken', 12],
	]) {
		const config = testConfig('/tmp')
		config[property] = ''
		assert.throws(
			() => validateEnvironment(config),
			error => error.exitCode === exitCode,
		)
	}
})

test('loadWorkerConfig exposes only non-sensitive build and health configuration', () => {
	const config = loadWorkerConfig({
		moduleDirname: '/app',
		env: {
			SYNC_HEALTH_STATE_PATH: '/run/canterbury-sync/health.json',
			CANTERBURY_VERSION: '1.2.3',
			CANTERBURY_REVISION: 'abc123',
		},
	})

	assert.equal(config.statePath, '/run/canterbury-sync/health.json')
	assert.equal(config.version, '1.2.3')
	assert.equal(config.revision, 'abc123')
})

test('describeCommand redacts the sync setup password', () => {
	assert.equal(
		describeCommand('ob', ['sync-setup', '--password', 'secret', '--vault', 'fake']),
		'ob sync-setup --password [redacted] --vault fake',
	)
})

test('runWorker completes initial sync before reporting continuous readiness', async () => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-worker-ready-'))
	const config = testConfig(directory)
	const commands = []
	let continuousChild
	let nextPid = 10_000

	const spawnProcess = (_command, args) => {
		commands.push(args)
		const child = createChild(nextPid)
		nextPid += 1
		queueMicrotask(() => {
			child.emit('spawn')
			if (args.at(-1) === '--continuous') {
				continuousChild = child
				return
			}
			const code = args[0] === 'sync-status' ? 3 : 0
			child.emit('close', code, null)
		})
		return child
	}

	const worker = runWorker({
		config,
		moduleDirname: directory,
		spawnProcess,
		heartbeatIntervalMs: 60_000,
		registerSignals: false,
		logger: { log() {} },
	})

	await waitFor(async () =>
		readHealth({ statePath: config.statePath, mode: 'ready', isAlive: () => true }),
	)
	assert.deepEqual(
		commands.map(args => [args[0], args.at(-1)]),
		[
			['sync-status', config.vaultPath],
			['sync-setup', 'test-device'],
			['sync', config.vaultPath],
			['sync', '--continuous'],
		],
	)
	assert.equal(await readFile(config.authTokenPath, 'utf8'), 'fake-token')
	assert.equal((await stat(config.authTokenPath)).mode & 0o777, 0o600)
	const stateText = await readFile(config.statePath, 'utf8')
	assert.doesNotMatch(stateText, /fake-password|fake-token|fake-vault/)

	continuousChild.emit('close', 1, null)
	await assert.rejects(worker, /ob sync .* --continuous exited with code 1/)
	assert.equal(await readHealth({ statePath: config.statePath, mode: 'live' }), false)
})

test('runWorker never becomes ready when initial sync fails', async () => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-worker-failed-'))
	const config = testConfig(directory)
	const spawnProcess = (_command, args) => {
		const child = createChild(20_000 + args.length)
		queueMicrotask(() => {
			child.emit('spawn')
			child.emit('close', args[0] === 'sync' ? 2 : 0, null)
		})
		return child
	}

	await assert.rejects(
		runWorker({
			config,
			moduleDirname: directory,
			spawnProcess,
			heartbeatIntervalMs: 60_000,
			registerSignals: false,
			logger: { log() {} },
		}),
		/ob sync .* exited with code 2/,
	)
	assert.equal(await readHealth({ statePath: config.statePath, mode: 'ready' }), false)
})

test('ProcessSupervisor rejects an unexpected clean continuous exit', async () => {
	const reporter = { update: () => Promise.resolve() }
	const child = createChild(25_000)
	const supervisor = new ProcessSupervisor({
		spawnProcess: () => {
			queueMicrotask(() => {
				child.emit('spawn')
				child.emit('close', 0, null)
			})
			return child
		},
		reporter,
	})

	await assert.rejects(
		supervisor.run('fake-ob', [], { rejectOnCleanExit: true }),
		/fake-ob exited unexpectedly/,
	)
})

test('ProcessSupervisor forwards shutdown and force kills an unresponsive child', async () => {
	const updates = []
	const reporter = { update: changes => (updates.push(changes), Promise.resolve()) }
	const child = createChild(30_000)
	const supervisor = new ProcessSupervisor({
		spawnProcess: () => {
			queueMicrotask(() => child.emit('spawn'))
			return child
		},
		reporter,
		shutdownTimeoutMs: 10,
	})
	const running = supervisor.run('fake-ob')

	await waitFor(() => updates.some(update => update.childPid === child.pid))
	await supervisor.shutdown('SIGTERM')
	await waitFor(() => child.signals.includes('SIGKILL'))
	assert.deepEqual(child.signals, ['SIGTERM', 'SIGKILL'])
	assert.ok(updates.some(update => update.phase === 'shutting-down'))

	child.emit('close', null, 'SIGKILL')
	await assert.rejects(running, /Received SIGTERM/)
	supervisor.close()
})

test('runWorker signal handlers mark not ready and forward SIGTERM', async () => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-worker-signal-'))
	const config = testConfig(directory)
	let continuousChild
	const spawnProcess = (_command, args) => {
		const child = createChild(40_000 + args.length)
		queueMicrotask(() => {
			child.emit('spawn')
			if (args.at(-1) === '--continuous') {
				continuousChild = child
				return
			}
			child.emit('close', 0, null)
		})
		return child
	}

	const worker = runWorker({
		config,
		moduleDirname: directory,
		spawnProcess,
		heartbeatIntervalMs: 60_000,
		shutdownTimeoutMs: 100,
		logger: { log() {} },
	})

	await waitFor(async () =>
		readHealth({ statePath: config.statePath, mode: 'ready', isAlive: () => true }),
	)
	process.emit('SIGTERM', 'SIGTERM')
	await waitFor(() => continuousChild.signals.includes('SIGTERM'))
	assert.equal(
		await readHealth({ statePath: config.statePath, mode: 'live', isAlive: () => true }),
		false,
	)

	continuousChild.emit('close', 0, null)
	await assert.rejects(worker, /Received SIGTERM/)
})
