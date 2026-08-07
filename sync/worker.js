import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { HealthReporter, removeHealthState } from './health.js'

export const EXIT_MISSING_VAULT_NAME = 10
export const EXIT_MISSING_VAULT_PASSWORD = 11
export const EXIT_MISSING_OBSIDIAN_AUTH_TOKEN = 12
export const DEFAULT_SHUTDOWN_TIMEOUT_MS = 20_000

const signalExitCodes = {
	SIGINT: 130,
	SIGTERM: 143,
}

const camelToKebab = value => value.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`)

export const obArgs = mapping =>
	Object.entries(mapping).flatMap(([name, value]) => [`--${camelToKebab(name)}`, value])

export const redactArgs = args =>
	args.map((arg, index) => (args[index - 1] === '--password' ? '[redacted]' : arg))

export const describeCommand = (command, args = []) => [command, ...redactArgs(args)].join(' ')

const createConfigError = (message, exitCode) => {
	const error = new Error(message)
	error.exitCode = exitCode
	return error
}

export const validateEnvironment = config => {
	if (!config.vaultName) {
		throw createConfigError('SYNC_VAULT_NAME is required', EXIT_MISSING_VAULT_NAME)
	}

	if (!config.vaultPassword) {
		throw createConfigError('SYNC_VAULT_PASSWORD is required', EXIT_MISSING_VAULT_PASSWORD)
	}

	if (!config.obsidianAuthToken) {
		throw createConfigError(
			'SYNC_OBSIDIAN_AUTH_TOKEN is required',
			EXIT_MISSING_OBSIDIAN_AUTH_TOKEN,
		)
	}
}

export const writeAuthToken = async ({ authTokenPath, obsidianAuthToken }) => {
	await mkdir(dirname(authTokenPath), { recursive: true })
	await writeFile(authTokenPath, obsidianAuthToken, { mode: 0o600 })
}

export const loadWorkerConfig = ({ env = process.env, moduleDirname }) => ({
	vaultPath: env.SYNC_VAULT_PATH || join(process.cwd(), 'vault'),
	vaultName: env.SYNC_VAULT_NAME || '',
	vaultPassword: env.SYNC_VAULT_PASSWORD || '',
	obsidianAuthToken: env.SYNC_OBSIDIAN_AUTH_TOKEN || '',
	deviceName: env.SYNC_DEVICE_NAME || 'canterbury-sync',
	statePath: env.SYNC_HEALTH_STATE_PATH || join(moduleDirname, '.runtime', 'health.json'),
	authTokenPath: join(homedir(), '.config', 'obsidian-headless', 'auth_token'),
	version: env.CANTERBURY_VERSION || 'dev',
	revision: env.CANTERBURY_REVISION || 'unknown',
})

export class ProcessSupervisor {
	#activeChildren = new Set()
	#forceTimer = null

	constructor({
		spawnProcess = spawn,
		reporter,
		shutdownTimeoutMs = DEFAULT_SHUTDOWN_TIMEOUT_MS,
		env = process.env,
		cwd,
	}) {
		this.spawnProcess = spawnProcess
		this.reporter = reporter
		this.shutdownTimeoutMs = shutdownTimeoutMs
		this.env = env
		this.cwd = cwd
		this.shutdownSignal = null
	}

	async run(
		command,
		args = [],
		{ rejectOnError = true, rejectOnCleanExit = false, label = command } = {},
	) {
		this.#throwIfShuttingDown()

		const result = await new Promise((resolve, reject) => {
			const child = this.spawnProcess(command, args, {
				cwd: this.cwd,
				stdio: 'inherit',
				env: this.env,
			})
			const commandDescription = describeCommand(label, args)
			let settled = false

			this.#activeChildren.add(child)

			child.once('spawn', () => {
				void this.reporter.update({ childPid: child.pid }).catch(error => reject(error))
			})

			child.once('error', error => {
				this.#activeChildren.delete(child)
				if (!settled) {
					settled = true
					reject(new Error(`Failed to start ${commandDescription}: ${error.message}`))
				}
			})

			child.once('close', (code, signal) => {
				this.#activeChildren.delete(child)
				void this.reporter.update({ childPid: null }).catch(() => {})

				if (settled) {
					return
				}

				settled = true
				if ((code === 0 && !rejectOnCleanExit) || !rejectOnError || this.shutdownSignal) {
					resolve({ code, signal })
					return
				}

				const reason = code === 0 ? 'exited unexpectedly' : `exited with code ${code ?? signal}`
				reject(new Error(`${commandDescription} ${reason}`))
			})
		})

		this.#throwIfShuttingDown()
		return result
	}

	async shutdown(signal) {
		if (this.shutdownSignal) {
			for (const child of this.#activeChildren) {
				child.kill('SIGKILL')
			}
			return
		}

		this.shutdownSignal = signal
		try {
			await this.reporter.update({ phase: 'shutting-down' })
		} catch (error) {
			console.error(`Failed to report sync shutdown: ${error.message}`)
		}

		for (const child of this.#activeChildren) {
			child.kill(signal)
		}

		this.#forceTimer = setTimeout(() => {
			for (const child of this.#activeChildren) {
				child.kill('SIGKILL')
			}
		}, this.shutdownTimeoutMs)
	}

	close() {
		if (this.#forceTimer !== null) {
			clearTimeout(this.#forceTimer)
			this.#forceTimer = null
		}
	}

	#throwIfShuttingDown() {
		if (!this.shutdownSignal) {
			return
		}

		const error = new Error(`Received ${this.shutdownSignal}`)
		error.exitCode = signalExitCodes[this.shutdownSignal] ?? 1
		error.silent = true
		throw error
	}
}

export const runWorker = async ({
	config,
	moduleDirname,
	spawnProcess,
	heartbeatIntervalMs,
	shutdownTimeoutMs,
	now,
	registerSignals = true,
	logger = console,
}) => {
	await removeHealthState(config.statePath)
	validateEnvironment(config)

	const reporter = new HealthReporter({
		statePath: config.statePath,
		version: config.version,
		revision: config.revision,
		heartbeatIntervalMs,
		now,
	})
	await reporter.start()

	const supervisor = new ProcessSupervisor({
		spawnProcess,
		reporter,
		shutdownTimeoutMs,
		env: process.env,
		cwd: moduleDirname,
	})
	const obPath = join(moduleDirname, 'node_modules', '.bin', 'ob')
	const signalHandlers = new Map()

	if (registerSignals) {
		for (const signal of ['SIGINT', 'SIGTERM']) {
			const handler = () => void supervisor.shutdown(signal)
			signalHandlers.set(signal, handler)
			process.on(signal, handler)
		}
	}

	const runOb = (command, args = [], options) =>
		supervisor.run(obPath, [command, ...args], { ...options, label: 'ob' })

	try {
		logger.log(
			`Starting Canterbury sync worker version=${config.version} revision=${config.revision}`,
		)
		await writeAuthToken(config)

		const status = await runOb('sync-status', obArgs({ path: config.vaultPath }), {
			rejectOnError: false,
		})
		if (status.code === 3) {
			await runOb(
				'sync-setup',
				obArgs({
					vault: config.vaultName,
					path: config.vaultPath,
					password: config.vaultPassword,
					deviceName: config.deviceName,
				}),
			)
		} else if (status.code !== 0) {
			throw new Error(`ob sync-status exited with code ${status.code ?? status.signal}`)
		}

		await reporter.update({ phase: 'initial-sync' })
		await runOb('sync', obArgs({ path: config.vaultPath }))
		await reporter.update({ initialSyncComplete: true })

		const continuous = supervisor.run(
			obPath,
			['sync', ...obArgs({ path: config.vaultPath }), '--continuous'],
			{ label: 'ob', rejectOnCleanExit: true },
		)
		await reporter.update({ phase: 'continuous-sync' })
		await continuous
	} catch (error) {
		if (!supervisor.shutdownSignal) {
			await reporter.update({ phase: 'failed', childPid: null })
		}
		throw error
	} finally {
		for (const [signal, handler] of signalHandlers) {
			process.removeListener(signal, handler)
		}
		supervisor.close()
		await reporter.stop()
	}
}
