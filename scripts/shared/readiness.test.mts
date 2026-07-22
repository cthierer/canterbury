import test from 'node:test'
import { waitForHTTP } from './readiness.mts'

test('waitForHTTP resolves when an endpoint returns a successful response', async () => {
	const originalFetch = globalThis.fetch
	globalThis.fetch = (() => Promise.resolve(new Response('ok', { status: 200 }))) as typeof fetch

	try {
		await waitForHTTP('http://127.0.0.1:1/ready', 'test HTTP server')
	} finally {
		globalThis.fetch = originalFetch
	}
})

test.todo('waitForTCP resolves when a TCP listener accepts connections')
