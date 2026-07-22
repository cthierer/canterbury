import assert from 'node:assert/strict'
import test from 'node:test'
import { brunoPhaseCommand, runBrunoPhase } from './bruno.mts'
import { brunoExe } from './paths.mts'

test('brunoPhaseCommand builds a tagged Bruno run command', () => {
	assert.deepEqual(
		brunoPhaseCommand('phase-tag', {
			collection: 'ignored-by-command-builder',
			envName: 'local',
			envVars: {
				DexBaseURI: 'http://127.0.0.1:5556/dex',
				DexClientID: 'pomerium',
			},
		}),
		[
			brunoExe(),
			'run',
			'--env',
			'local',
			'--env-var',
			'DexBaseURI=http://127.0.0.1:5556/dex',
			'--env-var',
			'DexClientID=pomerium',
			'--tags',
			'phase-tag',
			'--bail',
			'--noproxy',
		],
	)
})

test('runBrunoPhase fails when Bruno exits unsuccessfully', async () => {
	await assert.rejects(
		runBrunoPhase(
			'missing-tag',
			{
				collection: 'local-pomerium-smoke',
				envName: 'local',
			},
			{
				runner: async () => ({ code: 1, signal: null }),
			},
		),
		/Bruno phase missing-tag exited with code/,
	)
})

test('runBrunoPhase passes collection cwd and no-proxy environment to the runner', async () => {
	let cwd
	let noProxy

	await runBrunoPhase(
		'phase-tag',
		{
			collection: 'local-pomerium-smoke',
			envName: 'local',
			noProxy: '127.0.0.1,localhost',
		},
		{
			runner: async (_command, options) => {
				if (!options) {
					throw new Error('missing runner options')
				}
				cwd = options.cwd
				noProxy = options.env?.NO_PROXY
				return { code: 0, signal: null }
			},
		},
	)

	assert.match(String(cwd), /bruno[/\\]local-pomerium-smoke$/)
	assert.equal(noProxy, '127.0.0.1,localhost')
})
