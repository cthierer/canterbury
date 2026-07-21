import assert from 'node:assert/strict'
import { mkdtemp, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { loadEnvFile, parseEnvValues, readEnvFile } from './env.mts'

test('parseEnvValues parses simple KEY=value files', () => {
	assert.deepEqual(
		parseEnvValues(`
# comment
DEX_CLIENT_ID=pomerium
DEX_CLIENT_SECRET=abc=def
invalid line
EMPTY=
`),
		{
			DEX_CLIENT_ID: 'pomerium',
			DEX_CLIENT_SECRET: 'abc=def',
			EMPTY: '',
		},
	)
})

test('readEnvFile returns empty values for a missing file', async () => {
	assert.deepEqual(await readEnvFile(join(tmpdir(), `missing-${process.pid}.env`)), {})
})

test('loadEnvFile preserves existing environment values', async () => {
	const directory = await mkdtemp(join(tmpdir(), 'canterbury-env-test-'))
	const path = join(directory, 'local.env')
	await writeFile(path, 'EXISTING=file\nNEW=value\n')
	const env = {
		EXISTING: 'shell',
	}

	await loadEnvFile(path, { env })

	assert.deepEqual(env, {
		EXISTING: 'shell',
		NEW: 'value',
	})
})
