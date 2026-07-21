import assert from 'node:assert/strict'
import test from 'node:test'
import { parseJSON } from './http.mts'

test('parseJSON parses JSON responses', async () => {
	assert.deepEqual(await parseJSON(new Response('{"ok":true}')), { ok: true })
})

test('parseJSON returns null for empty responses', async () => {
	assert.equal(await parseJSON(new Response('')), null)
})

test('parseJSON preserves non-JSON text bodies', async () => {
	assert.deepEqual(await parseJSON(new Response('not json')), { raw: 'not json' })
})
