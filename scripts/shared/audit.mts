import { execFile } from 'node:child_process'
import { readFile, readdir } from 'node:fs/promises'
import { join } from 'node:path'
import { promisify } from 'node:util'
import { hasErrorCode } from './errors.mts'
import { sleep } from './processes.mts'

const execFileAsync = promisify(execFile)

/** Audit event shape used by smoke tests. */
export interface AuditEvent {
	readonly event_type?: unknown
}

/** Reads audit events from a backing audit log source. */
export type AuditEventReader = () => Promise<AuditEvent[]>

/** Parses newline-delimited JSON audit events, ignoring empty lines. */
export const parseJSONLines = (data: string): AuditEvent[] => {
	const events: AuditEvent[] = []
	for (const line of data.split('\n')) {
		if (line.trim() === '') {
			continue
		}

		events.push(JSON.parse(line) as AuditEvent)
	}

	return events
}

/** Recursively reads JSONL audit events from the host filesystem. */
export const readHostAuditEvents = async (directory: string): Promise<AuditEvent[]> => {
	let entries
	try {
		entries = await readdir(directory, { withFileTypes: true })
	} catch (error) {
		if (hasErrorCode(error, 'ENOENT')) {
			return []
		}

		throw error
	}

	const events: AuditEvent[] = []
	for (const entry of entries) {
		const path = join(directory, entry.name)
		if (entry.isDirectory()) {
			events.push(...(await readHostAuditEvents(path)))
			continue
		}

		if (entry.isFile() && entry.name.endsWith('.jsonl')) {
			events.push(...parseJSONLines(await readFile(path, 'utf8')))
		}
	}

	return events
}

/** Counts audit events of a specific type. */
export const countAuditEvents = (events: readonly AuditEvent[], eventType: string) => {
	return events.filter(event => event.event_type === eventType).length
}

export interface LocalAuditEventReaderOptions {
	readonly hostAuditRoot: string
	readonly dockerComposeCwd: string
	readonly containerName?: string
	readonly containerAuditRoot?: string
}

/** Creates a local audit event reader that falls back to Docker Compose for host permission issues. */
export const createLocalAuditEventReader = ({
	hostAuditRoot,
	dockerComposeCwd,
	containerName = 'vault-service',
	containerAuditRoot = '/audit',
}: LocalAuditEventReaderOptions): AuditEventReader => {
	return async () => {
		try {
			return await readHostAuditEvents(hostAuditRoot)
		} catch (error) {
			if (!hasErrorCode(error, 'EACCES') && !hasErrorCode(error, 'EPERM')) {
				throw error
			}

			return readContainerAuditEvents({ dockerComposeCwd, containerName, containerAuditRoot })
		}
	}
}

/** Reads JSONL audit events from a running Docker Compose service container. */
export const readContainerAuditEvents = async ({
	dockerComposeCwd,
	containerName = 'vault-service',
	containerAuditRoot = '/audit',
}: Omit<LocalAuditEventReaderOptions, 'hostAuditRoot'>): Promise<AuditEvent[]> => {
	const { stdout } = await execFileAsync(
		'docker',
		[
			'compose',
			'exec',
			'-T',
			containerName,
			'sh',
			'-c',
			`find ${containerAuditRoot} -type f -name "*.jsonl" -exec cat {} +`,
		],
		{ cwd: dockerComposeCwd },
	)

	return parseJSONLines(stdout)
}

/** Counts audit events of a specific type across readable audit log files. */
export const countAuditLogEvents = async (readEvents: AuditEventReader, eventType: string) => {
	return countAuditEvents(await readEvents(), eventType)
}

/** Waits until the audit log records a new event of the requested type. */
export const waitForAuditEvent = async (
	readEvents: AuditEventReader,
	eventType: string,
	previousCount: number,
	{ timeoutMillis = 10_000, intervalMillis = 250 } = {},
) => {
	const deadline = Date.now() + timeoutMillis

	while (Date.now() < deadline) {
		const currentCount = await countAuditLogEvents(readEvents, eventType)
		if (currentCount > previousCount) {
			return
		}

		await sleep(intervalMillis)
	}

	throw new Error(`timed out waiting for ${eventType} audit event`)
}

/** Runs an action and waits for the expected audit event to appear afterward. */
export const runWithAuditEvent = async (
	readEvents: AuditEventReader,
	eventType: string,
	action: () => Promise<void>,
) => {
	const previousCount = await countAuditLogEvents(readEvents, eventType)
	await action()
	await waitForAuditEvent(readEvents, eventType, previousCount)
}
