import { chmod, mkdir, readFile, rename, rm, writeFile } from 'node:fs/promises'
import { dirname } from 'node:path'

export const HEALTH_SCHEMA_VERSION = 1
export const DEFAULT_HEARTBEAT_INTERVAL_MS = 5_000
export const DEFAULT_HEARTBEAT_MAX_AGE_MS = 15_000

const livePhases = new Set(['initializing', 'initial-sync', 'continuous-sync'])

export const processIsAlive = pid => {
	if (!Number.isSafeInteger(pid) || pid <= 0) {
		return false
	}

	try {
		process.kill(pid, 0)
		return true
	} catch (error) {
		return error?.code === 'EPERM'
	}
}

const validState = state =>
	state !== null &&
	typeof state === 'object' &&
	state.schemaVersion === HEALTH_SCHEMA_VERSION &&
	Number.isSafeInteger(state.workerPid) &&
	state.workerPid > 0 &&
	(state.childPid === null || (Number.isSafeInteger(state.childPid) && state.childPid > 0)) &&
	typeof state.phase === 'string' &&
	typeof state.initialSyncComplete === 'boolean' &&
	Number.isFinite(state.updatedAt) &&
	typeof state.version === 'string' &&
	typeof state.revision === 'string'

export const evaluateHealth = ({
	state,
	mode = 'ready',
	now = Date.now(),
	maxAgeMs = DEFAULT_HEARTBEAT_MAX_AGE_MS,
	isAlive = processIsAlive,
}) => {
	if ((mode !== 'live' && mode !== 'ready') || !validState(state)) {
		return false
	}

	if (now - state.updatedAt < 0 || now - state.updatedAt > maxAgeMs) {
		return false
	}

	if (!livePhases.has(state.phase) || !isAlive(state.workerPid)) {
		return false
	}

	if (state.childPid !== null && !isAlive(state.childPid)) {
		return false
	}

	if (mode === 'live') {
		return true
	}

	return (
		state.phase === 'continuous-sync' &&
		state.initialSyncComplete &&
		state.childPid !== null &&
		isAlive(state.childPid)
	)
}

export const readHealth = async ({ statePath, mode, now, maxAgeMs, isAlive }) => {
	try {
		const state = JSON.parse(await readFile(statePath, 'utf8'))
		return evaluateHealth({ state, mode, now, maxAgeMs, isAlive })
	} catch {
		return false
	}
}

export const removeHealthState = statePath => rm(statePath, { force: true })

export class HealthReporter {
	#heartbeat = null
	#pendingWrite = Promise.resolve()
	#state

	constructor({
		statePath,
		version = 'dev',
		revision = 'unknown',
		workerPid = process.pid,
		now = Date.now,
		heartbeatIntervalMs = DEFAULT_HEARTBEAT_INTERVAL_MS,
		onError = error => console.error(`Failed to report sync health: ${error.message}`),
	}) {
		this.statePath = statePath
		this.now = now
		this.heartbeatIntervalMs = heartbeatIntervalMs
		this.onError = onError
		this.#state = {
			schemaVersion: HEALTH_SCHEMA_VERSION,
			workerPid,
			childPid: null,
			phase: 'initializing',
			initialSyncComplete: false,
			updatedAt: now(),
			version,
			revision,
		}
	}

	async start() {
		await mkdir(dirname(this.statePath), { recursive: true })
		await removeHealthState(this.statePath)
		await this.#write()

		this.#heartbeat = setInterval(() => {
			this.#state.updatedAt = this.now()
			void this.#write().catch(this.onError)
		}, this.heartbeatIntervalMs)
	}

	update(changes) {
		Object.assign(this.#state, changes, { updatedAt: this.now() })
		return this.#write()
	}

	async stop() {
		if (this.#heartbeat !== null) {
			clearInterval(this.#heartbeat)
			this.#heartbeat = null
		}

		await this.#pendingWrite
	}

	#write() {
		const contents = `${JSON.stringify(this.#state)}\n`
		const temporaryPath = `${this.statePath}.${this.#state.workerPid}.tmp`

		this.#pendingWrite = this.#pendingWrite.then(async () => {
			await writeFile(temporaryPath, contents, { mode: 0o600 })
			await chmod(temporaryPath, 0o600)
			await rename(temporaryPath, this.statePath)
		})

		return this.#pendingWrite
	}
}
