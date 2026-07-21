import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { countAuditEvents, parseJSONLines, readHostAuditEvents } from './audit.mts'

test('parseJSONLines parses non-empty JSONL rows', () => {
	assert.deepEqual(parseJSONLines('{"event_type":"a"}\n\n{"event_type":"b"}\n'), [
		{ event_type: 'a' },
		{ event_type: 'b' },
	])
})

test('countAuditEvents counts events by event_type', () => {
	assert.equal(
		countAuditEvents(
			[{ event_type: 'auth.failed' }, { event_type: 'vault.read.allowed' }],
			'auth.failed',
		),
		1,
	)
})

test('readHostAuditEvents recursively reads JSONL audit files', async () => {
	const root = join(tmpdir(), `canterbury-audit-test-${process.pid}-${Date.now()}`)
	const nested = join(root, 'nested')
	await mkdir(nested, { recursive: true })
	await writeFile(join(root, 'events.jsonl'), '{"event_type":"root"}\n')
	await writeFile(join(nested, 'events.jsonl'), '{"event_type":"nested"}\n')
	await writeFile(join(nested, 'ignore.txt'), '{"event_type":"ignored"}\n')

	assert.deepEqual(await readHostAuditEvents(root), [
		{ event_type: 'root' },
		{ event_type: 'nested' },
	])
})
