import assert from 'node:assert/strict'
import test from 'node:test'
import { MCPClient } from './mcpClient.mts'
import { postVault, readNoteBody } from './vaultClient.mts'

test('readNoteBody builds a VaultService ReadNote request body', () => {
	assert.deepEqual(readNoteBody('Projects/Canterbury.md'), {
		ref: {
			path: 'Projects/Canterbury.md',
		},
	})
})

test('postVault posts Connect JSON with an optional bearer token', async () => {
	const originalFetch = globalThis.fetch
	let request: Request | undefined
	globalThis.fetch = ((input, init) => {
		request = new Request(input, init)
		return Promise.resolve(new Response('{"ok":true}', { status: 200 }))
	}) as typeof fetch

	try {
		const response = await postVault(
			'https://vault.example.test',
			'ReadNote',
			readNoteBody('A.md'),
			'token',
		)

		assert.deepEqual(response, {
			status: 200,
			body: { ok: true },
		})
		assert.equal(
			request?.url,
			'https://vault.example.test/canterbury.vault.v1.VaultService/ReadNote',
		)
		assert.equal(request?.headers.get('authorization'), 'Bearer token')
		assert.equal(request?.headers.get('content-type'), 'application/json')
		assert.equal(await request?.text(), JSON.stringify(readNoteBody('A.md')))
	} finally {
		globalThis.fetch = originalFetch
	}
})

test('MCPClient posts incrementing JSON-RPC requests', async () => {
	const originalFetch = globalThis.fetch
	const bodies: unknown[] = []
	globalThis.fetch = ((input, init) => {
		assert.equal(String(input), 'https://mcp.example.test/mcp')
		bodies.push(JSON.parse(String(init?.body)))
		return Promise.resolve(new Response('{"result":{}}', { status: 200 }))
	}) as typeof fetch

	try {
		const client = new MCPClient('https://mcp.example.test')
		await client.post('tools/list', {}, 'token')
		await client.post('tools/call', { name: 'read_note' }, 'token')

		assert.deepEqual(bodies, [
			{ jsonrpc: '2.0', id: 1, method: 'tools/list', params: {} },
			{ jsonrpc: '2.0', id: 2, method: 'tools/call', params: { name: 'read_note' } },
		])
	} finally {
		globalThis.fetch = originalFetch
	}
})
