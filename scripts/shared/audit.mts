import { readFile, readdir } from 'node:fs/promises'
import { join } from 'node:path'
import { hasErrorCode } from './errors.mts'

/** Audit event shape used by smoke tests. */
export interface AuditEvent {
	readonly event_type?: unknown
}

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
