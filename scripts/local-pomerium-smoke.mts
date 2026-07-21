import { existsSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { join } from 'node:path'
import { createLocalAuditEventReader, runWithAuditEvent } from './shared/audit.mts'
import { runBrunoPhase } from './shared/bruno.mts'
import { loadEnvFile } from './shared/env.mts'
import { localPomeriumDir, repoRoot } from './shared/paths.mts'
import { waitForHTTP } from './shared/readiness.mts'

const root = repoRoot()
const pomeriumDir = localPomeriumDir()
const readinessOptions = {
	timeoutMillis: 30_000,
	intervalMillis: 500,
	requestTimeoutMillis: 1000,
}

/** Restarts the script with the generated Pomerium CA when HTTPS verification needs it. */
const ensurePomeriumCACert = () => {
	if (!pomeriumBaseURL.startsWith('https://') || process.env.NODE_EXTRA_CA_CERTS) {
		return
	}

	if (!existsSync(pomeriumCACert)) {
		throw new Error(
			`missing Pomerium CA certificate at ${pomeriumCACert}; run scripts/setup-local-pomerium.mts first`,
		)
	}

	const result = spawnSync(process.execPath, process.argv.slice(1), {
		stdio: 'inherit',
		env: {
			...process.env,
			NODE_EXTRA_CA_CERTS: pomeriumCACert,
		},
	})

	process.exit(result.status ?? 1)
}

await loadEnvFile(join(pomeriumDir, 'local.env'))

const dexBaseURL = process.env.DEX_BASE_URL ?? 'http://127.0.0.1:5556/dex'
const pomeriumBaseURL = process.env.POMERIUM_BASE_URL ?? 'https://vault.localhost.pomerium.io:8443'
const pomeriumCACert =
	process.env.POMERIUM_CA_CERT ?? join(pomeriumDir, '.generated', 'certs', 'pomerium-local.crt')
const auditRoot = process.env.VAULT_SERVICE_AUDIT_ROOT ?? join(root, 'audit')
const dexClientID = process.env.DEX_CLIENT_ID ?? 'pomerium'
const dexClientSecret = process.env.DEX_CLIENT_SECRET
const testPassword = process.env.DEX_TEST_PASSWORD ?? 'password'
const brunoOptions = {
	collection: 'local-pomerium-smoke',
	envName: 'local',
	envVars: {
		DexBaseURI: dexBaseURL,
		PomeriumBaseURI: pomeriumBaseURL,
		DexClientID: dexClientID,
		DexClientSecret: dexClientSecret ?? '',
		DexTestPassword: testPassword,
	},
	noProxy: '127.0.0.1,localhost,.localhost.pomerium.io,localhost.pomerium.io',
}
const readAuditEvents = createLocalAuditEventReader({
	hostAuditRoot: auditRoot,
	dockerComposeCwd: root,
})

if (!dexClientSecret) {
	throw new Error('missing DEX_CLIENT_SECRET; run scripts/setup-local-pomerium.mts first')
}

ensurePomeriumCACert()

await waitForHTTP(
	`${dexBaseURL}/.well-known/openid-configuration`,
	'Dex discovery',
	readinessOptions,
)
await waitForHTTP(
	`${pomeriumBaseURL}/.well-known/pomerium/jwks.json`,
	'Pomerium JWKS',
	readinessOptions,
)

await runBrunoPhase('pomerium-vault-auth', brunoOptions)
await runWithAuditEvent(readAuditEvents, 'auth.failed', () =>
	runBrunoPhase('pomerium-auth-failed-audit', brunoOptions),
)
await runWithAuditEvent(readAuditEvents, 'vault.read.allowed', () =>
	runBrunoPhase('pomerium-mcp-read-audit', brunoOptions),
)
await runWithAuditEvent(readAuditEvents, 'vault.search.completed', () =>
	runBrunoPhase('pomerium-mcp-search-audit', brunoOptions),
)
await runBrunoPhase('pomerium-mcp-authz', brunoOptions)

console.log('local Pomerium smoke passed')
