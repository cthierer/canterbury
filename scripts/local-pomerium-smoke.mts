import { existsSync } from 'node:fs'
import { execFile, spawnSync } from 'node:child_process'
import { join } from 'node:path'
import { promisify } from 'node:util'
import {
	parseJSONLines,
	readHostAuditEvents,
	countAuditEvents as countEvents,
	type AuditEvent,
} from './shared/audit.mts'
import { hasErrorCode, isRecord } from './shared/errors.mts'
import { loadEnvFile } from './shared/env.mts'
import { parseJSON, type HTTPResponse } from './shared/http.mts'
import { MCPClient } from './shared/mcpClient.mts'
import { localPomeriumDir, repoRoot } from './shared/paths.mts'
import { sleep } from './shared/processes.mts'
import { waitForHTTP } from './shared/readiness.mts'
import { postVault as postVaultRequest, readNoteBody } from './shared/vaultClient.mts'

/** Function used to validate a successful MCP tool result. */
type MCPResultValidator = (result: Record<string, unknown>) => boolean

const execFileAsync = promisify(execFile)
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

/** Returns the Dex client secret after the startup validation has run. */
const getDexClientSecret = () => {
	if (!dexClientSecret) {
		throw new Error('missing DEX_CLIENT_SECRET; run scripts/setup-local-pomerium.mts first')
	}

	return dexClientSecret
}

/** Mints a password-grant ID token for one local Dex test user. */
const mintIDToken = async (username: string) => {
	const body = new URLSearchParams({
		grant_type: 'password',
		scope: 'openid profile email',
		client_id: dexClientID,
		client_secret: getDexClientSecret(),
		username,
		password: testPassword,
	})

	const response = await fetch(`${dexBaseURL}/token`, {
		method: 'POST',
		headers: {
			'content-type': 'application/x-www-form-urlencoded',
		},
		body,
	})

	const payload = await parseJSON(response)
	if (!isRecord(payload)) {
		throw new Error(
			`mint Dex token for ${username}: unexpected response ${JSON.stringify(payload)}`,
		)
	}

	if (!response.ok) {
		throw new Error(
			`mint Dex token for ${username}: HTTP ${response.status} ${JSON.stringify(payload)}`,
		)
	}

	if (typeof payload.id_token !== 'string' || payload.id_token.length === 0) {
		throw new Error(`mint Dex token for ${username}: missing id_token`)
	}

	return payload.id_token
}

/** Verifies that an authorized token can read the requested note path. */
const assertReadSucceeds = async (token: string, path: string) => {
	const response = await postVault('ReadNote', readNoteBody(path), token)
	if (response.status !== 200) {
		throw new Error(`read ${path}: HTTP ${response.status} ${JSON.stringify(response.body)}`)
	}

	const notePath =
		isRecord(response.body) &&
		isRecord(response.body.note) &&
		isRecord(response.body.note.ref) &&
		response.body.note.ref.path
	if (notePath !== path) {
		throw new Error(`read ${path}: unexpected response ${JSON.stringify(response.body)}`)
	}
}

/** Verifies that a token is rejected with the expected Connect error response. */
const assertReadDenied = async (token: string, path: string, status: number, code: string) => {
	const response = await postVault('ReadNote', readNoteBody(path), token)
	const responseCode = isRecord(response.body) ? response.body.code : undefined
	if (response.status !== status || responseCode !== code) {
		throw new Error(
			`read ${path}: got HTTP ${response.status} ${JSON.stringify(response.body)}, want HTTP ${status} ${code}`,
		)
	}
}

/** Posts a Connect JSON request through Pomerium to the vault service. */
const postVault = async (method: string, body: unknown, token?: string) =>
	postVaultRequest(pomeriumBaseURL, method, body, token)

/** Verifies that the Pomerium-proxied MCP server exposes only the expected tools. */
const assertMCPTools = async (token: string) => {
	// The MCP handler runs in stateless Streamable HTTP mode, so this smoke test
	// intentionally probes the raw JSON-RPC route without a persisted handshake.
	const response = await postMCP('tools/list', {}, token)
	const responseBody = isRecord(response.body) ? response.body : {}
	if (response.status !== 200 || responseBody.error) {
		throw new Error(`list MCP tools: HTTP ${response.status} ${JSON.stringify(response.body)}`)
	}

	const result = isRecord(responseBody.result) ? responseBody.result : {}
	const tools = Array.isArray(result.tools) ? result.tools : []
	const names = tools
		.map(tool => {
			return isRecord(tool) && typeof tool.name === 'string' ? tool.name : undefined
		})
		.filter((name): name is string => name !== undefined)
		.sort()
	if (JSON.stringify(names) !== JSON.stringify(['read_note', 'search_notes'])) {
		throw new Error(`list MCP tools: unexpected names ${JSON.stringify(names)}`)
	}
}

/** Verifies that an MCP tool call succeeds and returns the expected structured result. */
const assertMCPCallSucceeds = async (
	token: string,
	name: string,
	arguments_: unknown,
	validate: MCPResultValidator,
) => {
	const response = await postMCP('tools/call', { name, arguments: arguments_ }, token)
	const responseBody = isRecord(response.body) ? response.body : {}
	const result = isRecord(responseBody.result) ? responseBody.result : undefined
	if (
		response.status !== 200 ||
		responseBody.error ||
		!result ||
		result.isError ||
		!validate(result)
	) {
		throw new Error(
			`call MCP tool ${name}: HTTP ${response.status} ${JSON.stringify(response.body)}`,
		)
	}
}

/** Verifies that an MCP tool call returns a tool-level failure. */
const assertMCPCallFails = async (token: string, name: string, arguments_: unknown) => {
	const response = await postMCP('tools/call', { name, arguments: arguments_ }, token)
	const responseBody = isRecord(response.body) ? response.body : {}
	const result = isRecord(responseBody.result) ? responseBody.result : {}
	if (response.status !== 200 || responseBody.error || result.isError !== true) {
		throw new Error(
			`call denied MCP tool ${name}: HTTP ${response.status} ${JSON.stringify(response.body)}`,
		)
	}
}

/** Posts one JSON-RPC request through Pomerium to the MCP endpoint. */
const postMCP = async (method: string, params: unknown, token?: string): Promise<HTTPResponse> => {
	return mcpClient.post(method, params, token)
}

/** Waits until the audit log records a new authentication failure event. */
const waitForAuditFailure = async (previousCount: number) => {
	return waitForAuditEvent('auth.failed', previousCount)
}

/** Waits until the audit log records a new event of the requested type. */
const waitForAuditEvent = async (eventType: string, previousCount: number) => {
	const deadline = Date.now() + 10_000

	while (Date.now() < deadline) {
		const currentCount = await countAuditEvents(eventType)
		if (currentCount > previousCount) {
			return
		}

		await sleep(250)
	}

	throw new Error(`timed out waiting for ${eventType} audit event`)
}

/** Counts auth.failed events across all readable audit log files. */
const countAuthFailures = async () => {
	return countAuditEvents('auth.failed')
}

/** Counts audit events of a specific type across all readable audit log files. */
const countAuditEvents = async (eventType: string) => {
	const events = await readAuditEvents()
	return countEvents(events, eventType)
}

/** Reads audit events from the host path, falling back to the container for permission issues. */
const readAuditEvents = async (): Promise<AuditEvent[]> => {
	try {
		return await readHostAuditEvents(auditRoot)
	} catch (error) {
		if (!hasErrorCode(error, 'EACCES') && !hasErrorCode(error, 'EPERM')) {
			throw error
		}

		return readContainerAuditEvents()
	}
}

/** Reads JSONL audit events from the running vault-service container. */
const readContainerAuditEvents = async (): Promise<AuditEvent[]> => {
	const { stdout } = await execFileAsync(
		'docker',
		[
			'compose',
			'exec',
			'-T',
			'vault-service',
			'sh',
			'-c',
			'find /audit -type f -name "*.jsonl" -exec cat {} +',
		],
		{ cwd: root },
	)

	return parseJSONLines(stdout)
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
const mcpClient = new MCPClient(pomeriumBaseURL)

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

const agentToken = await mintIDToken('agent@canterbury.local')
const publicToken = await mintIDToken('public@canterbury.local')
const unmappedToken = await mintIDToken('unmapped@canterbury.local')

await assertReadSucceeds(agentToken, 'Projects/Canterbury.md')
await assertReadSucceeds(publicToken, 'Public/Service Brief.md')
await assertReadDenied(publicToken, 'Projects/Canterbury.md', 403, 'permission_denied')

const authFailuresBefore = await countAuthFailures()
await assertReadDenied(unmappedToken, 'Projects/Canterbury.md', 401, 'unauthenticated')
await waitForAuditFailure(authFailuresBefore)

const missingBearer = await postVault('ReadNote', readNoteBody('Projects/Canterbury.md'))
if (missingBearer.status >= 200 && missingBearer.status < 300) {
	throw new Error(`missing bearer request unexpectedly returned HTTP ${missingBearer.status}`)
}

await assertMCPTools(agentToken)

const readAuditsBefore = await countAuditEvents('vault.read.allowed')
await assertMCPCallSucceeds(
	agentToken,
	'read_note',
	readNoteBody('Projects/Canterbury.md'),
	result => {
		const structuredContent = isRecord(result.structuredContent) ? result.structuredContent : {}
		const note = isRecord(structuredContent.note) ? structuredContent.note : {}
		const ref = isRecord(note.ref) ? note.ref : {}
		return ref.path === 'Projects/Canterbury.md'
	},
)
await waitForAuditEvent('vault.read.allowed', readAuditsBefore)

const searchAuditsBefore = await countAuditEvents('vault.search.completed')
await assertMCPCallSucceeds(
	agentToken,
	'search_notes',
	{ query: { text: 'Canterbury' }, filter: { includePathPrefixes: ['Projects'] } },
	result => {
		const structuredContent = isRecord(result.structuredContent) ? result.structuredContent : {}
		return Array.isArray(structuredContent.results)
	},
)
await waitForAuditEvent('vault.search.completed', searchAuditsBefore)

await assertMCPCallFails(publicToken, 'read_note', readNoteBody('Projects/Canterbury.md'))
await assertMCPCallSucceeds(
	publicToken,
	'search_notes',
	{ query: { text: 'Service' }, filter: { includePathPrefixes: ['Public'] } },
	result => {
		const structuredContent = isRecord(result.structuredContent) ? result.structuredContent : {}
		const results = Array.isArray(structuredContent.results) ? structuredContent.results : []
		const firstResult = isRecord(results[0]) ? results[0] : {}
		const ref = isRecord(firstResult.ref) ? firstResult.ref : {}
		return ref.path === 'Public/Service Brief.md'
	},
)
await assertMCPCallFails(unmappedToken, 'read_note', readNoteBody('Projects/Canterbury.md'))
await assertMCPCallFails(unmappedToken, 'search_notes', { query: { text: 'Canterbury' } })

const missingMCPBearer = await postMCP('tools/list', {})
if (missingMCPBearer.status >= 200 && missingMCPBearer.status < 300) {
	throw new Error(
		`missing MCP bearer request unexpectedly returned HTTP ${missingMCPBearer.status}`,
	)
}

console.log('local Pomerium smoke passed')
