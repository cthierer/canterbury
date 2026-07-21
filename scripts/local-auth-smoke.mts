import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { repoRoot, brunoExe, brunoTestDir } from './shared/paths.mts'
import { ProcessGroup, runForeground } from './shared/processes.mts'
import { startDevAuth, startVaultService } from './shared/localStack.mts'

const devAuthAddress = process.env.DEV_AUTH_ADDR ?? '127.0.0.1:50052'
const vaultAddress = process.env.VAULT_SERVICE_ADDR ?? '127.0.0.1:50051'
const auditRoot = await mkdtemp(join(tmpdir(), 'canterbury-smoke-audit-'))
const processes = new ProcessGroup({ cwd: repoRoot() })

let cleanupStarted = false

/** Performs idempotent process shutdown and temporary audit-directory cleanup. */
const cleanup = async () => {
	if (cleanupStarted) {
		return
	}

	cleanupStarted = true

	const errors = []

	try {
		await processes.shutdown()
	} catch (err) {
		errors.push(err)
	}

	try {
		await rm(auditRoot, { recursive: true, force: true })
	} catch (err) {
		errors.push(err)
	}

	if (errors.length > 0) {
		throw new AggregateError(errors, 'failed to cleanup smoke test run')
	}
}

process.once('SIGINT', () => {
	process.exitCode = 130
	void cleanup().finally(() => process.exit(process.exitCode))
})

process.once('SIGTERM', () => {
	process.exitCode = 143
	void cleanup().finally(() => process.exit(process.exitCode))
})

try {
	const devAuth = await startDevAuth(processes, devAuthAddress)
	const vaultService = await startVaultService(processes, vaultAddress, devAuth.jwksURL, auditRoot)

	const bruBin = brunoExe()
	const result = await runForeground(
		[
			bruBin,
			'run',
			'--env',
			'local',
			'--env-var',
			`DevAuthBaseURI=${devAuth.baseURL}`,
			'--env-var',
			`VaultBaseURI=${vaultService.baseURL}`,
			'--bail',
			'--noproxy',
		],
		{
			cwd: brunoTestDir('local-auth-smoke'),
			env: {
				...process.env,
				NO_PROXY: '127.0.0.1,localhost',
				no_proxy: '127.0.0.1,localhost',
			},
		},
	)

	process.exitCode = result.code ?? 1
} finally {
	await cleanup()
}
