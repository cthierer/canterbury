import { existsSync } from 'node:fs'
import { readFile, readdir } from 'node:fs/promises'
import { execFile, spawnSync } from 'node:child_process'
import { dirname, join } from 'node:path'
import { promisify } from 'node:util'
import { fileURLToPath } from 'node:url'

const execFileAsync = promisify(execFile)
const root = dirname(dirname(fileURLToPath(import.meta.url)))
const localPomeriumDir = join(root, 'deploy', 'local-pomerium')
await loadLocalEnv(join(localPomeriumDir, 'local.env'))

const dexBaseURL = process.env.DEX_BASE_URL ?? 'http://127.0.0.1:5556/dex'
const pomeriumBaseURL = process.env.POMERIUM_BASE_URL ?? 'https://vault.localhost.pomerium.io:8443'
const pomeriumCACert =
	process.env.POMERIUM_CA_CERT ??
	join(localPomeriumDir, '.generated', 'certs', 'pomerium-local.crt')
const auditRoot = process.env.VAULT_SERVICE_AUDIT_ROOT ?? join(root, 'audit')
const dexClientID = process.env.DEX_CLIENT_ID ?? 'pomerium'
const dexClientSecret = process.env.DEX_CLIENT_SECRET
const testPassword = process.env.DEX_TEST_PASSWORD ?? 'password'
let mcpRequestID = 0

if (!dexClientSecret) {
	throw new Error('missing DEX_CLIENT_SECRET; run scripts/setup-local-pomerium.mjs first')
}

ensurePomeriumCACert()

await waitForHTTP(`${dexBaseURL}/.well-known/openid-configuration`, 'Dex discovery')
await waitForHTTP(`${pomeriumBaseURL}/.well-known/pomerium/jwks.json`, 'Pomerium JWKS')

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
		return result?.structuredContent?.note?.ref?.path === 'Projects/Canterbury.md'
	},
)
await waitForAuditEvent('vault.read.allowed', readAuditsBefore)

const searchAuditsBefore = await countAuditEvents('vault.search.completed')
await assertMCPCallSucceeds(
	agentToken,
	'search_notes',
	{ query: { text: 'Canterbury' }, filter: { includePathPrefixes: ['Projects'] } },
	result => Array.isArray(result?.structuredContent?.results),
)
await waitForAuditEvent('vault.search.completed', searchAuditsBefore)

await assertMCPCallFails(publicToken, 'read_note', readNoteBody('Projects/Canterbury.md'))
await assertMCPCallSucceeds(
	publicToken,
	'search_notes',
	{ query: { text: 'Service' }, filter: { includePathPrefixes: ['Public'] } },
	result => result?.structuredContent?.results?.[0]?.ref?.path === 'Public/Service Brief.md',
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

async function loadLocalEnv(path) {
	let data
	try {
		data = await readFile(path, 'utf8')
	} catch (error) {
		if (error.code === 'ENOENT') {
			return
		}

		throw error
	}

	for (const line of data.split('\n')) {
		const trimmed = line.trim()
		if (trimmed === '' || trimmed.startsWith('#')) {
			continue
		}

		const match = /^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/.exec(trimmed)
		if (!match || process.env[match[1]] !== undefined) {
			continue
		}

		process.env[match[1]] = match[2]
	}
}

function ensurePomeriumCACert() {
	if (!pomeriumBaseURL.startsWith('https://') || process.env.NODE_EXTRA_CA_CERTS) {
		return
	}

	if (!existsSync(pomeriumCACert)) {
		throw new Error(
			`missing Pomerium CA certificate at ${pomeriumCACert}; run scripts/setup-local-pomerium.mjs first`,
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

async function mintIDToken(username) {
	const body = new URLSearchParams({
		grant_type: 'password',
		scope: 'openid profile email',
		client_id: dexClientID,
		client_secret: dexClientSecret,
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

async function assertReadSucceeds(token, path) {
	const response = await postVault('ReadNote', readNoteBody(path), token)
	if (response.status !== 200) {
		throw new Error(`read ${path}: HTTP ${response.status} ${JSON.stringify(response.body)}`)
	}

	if (response.body?.note?.ref?.path !== path) {
		throw new Error(`read ${path}: unexpected response ${JSON.stringify(response.body)}`)
	}
}

async function assertReadDenied(token, path, status, code) {
	const response = await postVault('ReadNote', readNoteBody(path), token)
	if (response.status !== status || response.body?.code !== code) {
		throw new Error(
			`read ${path}: got HTTP ${response.status} ${JSON.stringify(response.body)}, want HTTP ${status} ${code}`,
		)
	}
}

async function postVault(method, body, token) {
	const headers = {
		'content-type': 'application/json',
	}

	if (token) {
		headers.authorization = `Bearer ${token}`
	}

	const response = await fetch(`${pomeriumBaseURL}/canterbury.vault.v1.VaultService/${method}`, {
		method: 'POST',
		headers,
		body: JSON.stringify(body),
		redirect: 'manual',
	})

	return {
		status: response.status,
		body: await parseJSON(response),
	}
}

async function assertMCPTools(token) {
	const response = await postMCP('tools/list', {}, token)
	if (response.status !== 200 || response.body?.error) {
		throw new Error(`list MCP tools: HTTP ${response.status} ${JSON.stringify(response.body)}`)
	}

	const names = response.body?.result?.tools?.map(tool => tool.name).sort()
	if (JSON.stringify(names) !== JSON.stringify(['read_note', 'search_notes'])) {
		throw new Error(`list MCP tools: unexpected names ${JSON.stringify(names)}`)
	}
}

async function assertMCPCallSucceeds(token, name, arguments_, validate) {
	const response = await postMCP('tools/call', { name, arguments: arguments_ }, token)
	const result = response.body?.result
	if (response.status !== 200 || response.body?.error || result?.isError || !validate(result)) {
		throw new Error(
			`call MCP tool ${name}: HTTP ${response.status} ${JSON.stringify(response.body)}`,
		)
	}
}

async function assertMCPCallFails(token, name, arguments_) {
	const response = await postMCP('tools/call', { name, arguments: arguments_ }, token)
	if (response.status !== 200 || response.body?.error || response.body?.result?.isError !== true) {
		throw new Error(
			`call denied MCP tool ${name}: HTTP ${response.status} ${JSON.stringify(response.body)}`,
		)
	}
}

async function postMCP(method, params, token) {
	const headers = {
		accept: 'application/json, text/event-stream',
		'content-type': 'application/json',
		'mcp-protocol-version': '2025-06-18',
	}

	if (token) {
		headers.authorization = `Bearer ${token}`
	}

	const response = await fetch(`${pomeriumBaseURL}/mcp`, {
		method: 'POST',
		headers,
		body: JSON.stringify({
			jsonrpc: '2.0',
			id: ++mcpRequestID,
			method,
			params,
		}),
		redirect: 'manual',
	})

	return {
		status: response.status,
		body: await parseJSON(response),
	}
}

function readNoteBody(path) {
	return {
		ref: {
			path,
		},
	}
}

async function waitForHTTP(url, label) {
	const deadline = Date.now() + 30_000
	let lastError

	while (Date.now() < deadline) {
		try {
			const response = await fetch(url, { signal: AbortSignal.timeout(1000) })
			if (response.ok) {
				return
			}

			lastError = new Error(`${label} returned HTTP ${response.status}`)
		} catch (error) {
			lastError = error
		}

		await sleep(500)
	}

	throw new Error(`timed out waiting for ${label}: ${lastError?.message ?? 'no response'}`)
}

async function waitForAuditFailure(previousCount) {
	return waitForAuditEvent('auth.failed', previousCount)
}

async function waitForAuditEvent(eventType, previousCount) {
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

async function countAuthFailures() {
	return countAuditEvents('auth.failed')
}

async function countAuditEvents(eventType) {
	const events = await readAuditEvents()
	return events.filter(event => event.event_type === eventType).length
}

async function readAuditEvents() {
	try {
		return await readHostAuditEvents(auditRoot)
	} catch (error) {
		if (error.code !== 'EACCES' && error.code !== 'EPERM') {
			throw error
		}

		return readContainerAuditEvents()
	}
}

async function readHostAuditEvents(directory) {
	let entries
	try {
		entries = await readdir(directory, { withFileTypes: true })
	} catch (error) {
		if (error.code === 'ENOENT') {
			return []
		}

		throw error
	}

	const events = []
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

async function readContainerAuditEvents() {
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

function parseJSONLines(data) {
	const events = []
	for (const line of data.split('\n')) {
		if (line.trim() === '') {
			continue
		}

		events.push(JSON.parse(line))
	}

	return events
}

async function parseJSON(response) {
	const text = await response.text()
	if (text.trim() === '') {
		return null
	}

	try {
		return JSON.parse(text)
	} catch {
		return { raw: text }
	}
}

function sleep(ms) {
	return new Promise(resolve => {
		setTimeout(resolve, ms)
	})
}
