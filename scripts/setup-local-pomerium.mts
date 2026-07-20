#!/usr/bin/env node
import { randomBytes } from 'node:crypto'
import { chmod, mkdir, readFile, rename, unlink, writeFile } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

/** Environment-file values parsed from KEY=value lines. */
type EnvValues = Record<string, string>

const scriptDir = dirname(fileURLToPath(import.meta.url))
const rootDir = dirname(scriptDir)
const localDir = join(rootDir, 'deploy', 'local-pomerium')
const generatedDir = join(localDir, '.generated')
const envFile = join(localDir, 'local.env')
const auditDir = join(rootDir, 'audit')
const certDir = join(generatedDir, 'certs')
const keyDir = join(generatedDir, 'keys')

process.umask(0o077)

/** Exits early with a clear message when the local OpenSSL CLI is unavailable. */
const ensureOpenSSL = () => {
	try {
		execFileSync('openssl', ['version'], { stdio: 'ignore' })
	} catch {
		console.error('Missing required command: openssl. Install OpenSSL and retry.')
		process.exit(1)
	}
}

/** Generates a base64-encoded 256-bit secret for local-only credentials. */
const randomBase64 = () => {
	return randomBytes(32).toString('base64')
}

/** Builds the UID/GID environment prefix used by the local Docker Compose stack. */
const composeUserEnv = () => {
	if (typeof process.getuid === 'function' && typeof process.getgid === 'function') {
		return `CANTERBURY_UID=${process.getuid()} CANTERBURY_GID=${process.getgid()}`
	}

	return 'CANTERBURY_UID=$(id -u) CANTERBURY_GID=$(id -g)'
}

/** Checks Node-style thrown values for a specific filesystem or process error code. */
const hasErrorCode = (error: unknown, code: string) => {
	return (
		typeof error === 'object' &&
		error !== null &&
		'code' in error &&
		(error as { code?: unknown }).code === code
	)
}

/** Returns a useful message for unknown caught values. */
const getErrorMessage = (error: unknown) => {
	return error instanceof Error ? error.message : String(error)
}

/** Reads an optional local.env file without overwriting missing files as errors. */
const readEnvFile = async (path: string): Promise<EnvValues> => {
	let data
	try {
		data = await readFile(path, 'utf8')
	} catch (error) {
		if (hasErrorCode(error, 'ENOENT')) {
			return {}
		}

		throw new Error(`Failed to read local environment file at ${path}: ${getErrorMessage(error)}`, {
			cause: error,
		})
	}

	const values: EnvValues = {}
	for (const line of data.split('\n')) {
		const trimmed = line.trim()
		if (trimmed === '' || trimmed.startsWith('#')) {
			continue
		}

		const separator = trimmed.indexOf('=')
		if (separator === -1) {
			continue
		}

		values[trimmed.slice(0, separator)] = trimmed.slice(separator + 1)
	}

	return values
}

/** Renders a generated config file by replacing explicit template placeholders. */
const renderTemplate = async (
	source: string,
	destination: string,
	replacements: EnvValues,
	mode: number,
) => {
	let content = await readFile(source, 'utf8')
	for (const [placeholder, value] of Object.entries(replacements)) {
		content = content.replaceAll(placeholder, value)
	}

	await writeFileWithMode(destination, content, mode)
}

/** Writes sensitive local configuration with owner-only file permissions. */
const writeSecretFile = async (destination: string, content: string) => {
	await writeFileWithMode(destination, content, 0o600)
}

/** Atomically writes a file and applies the requested POSIX mode after rename. */
const writeFileWithMode = async (destination: string, content: string, mode: number) => {
	const temporary = `${destination}.${process.pid}.${randomBytes(4).toString('hex')}.tmp`
	try {
		await writeFile(temporary, content, { mode })
		await rename(temporary, destination)
		await chmod(destination, mode)
	} catch (error) {
		await unlink(temporary).catch(() => undefined)
		throw error
	}
}

ensureOpenSSL()

await mkdir(certDir, { recursive: true })
await mkdir(keyDir, { recursive: true })
await mkdir(auditDir, { recursive: true })

const currentEnv = await readEnvFile(envFile)
const localEnv = {
	DEX_CLIENT_ID: currentEnv.DEX_CLIENT_ID ?? 'pomerium',
	DEX_CLIENT_SECRET: currentEnv.DEX_CLIENT_SECRET ?? randomBase64(),
	DEX_TEST_PASSWORD: currentEnv.DEX_TEST_PASSWORD ?? 'password',
	VAULT_SERVICE_AUDIT_HMAC_KEY: currentEnv.VAULT_SERVICE_AUDIT_HMAC_KEY ?? randomBase64(),
	POMERIUM_COOKIE_SECRET: currentEnv.POMERIUM_COOKIE_SECRET ?? randomBase64(),
	POMERIUM_SHARED_SECRET: currentEnv.POMERIUM_SHARED_SECRET ?? randomBase64(),
}

await writeSecretFile(
	envFile,
	`${Object.entries(localEnv)
		.map(([key, value]) => `${key}=${value}`)
		.join('\n')}\n`,
)

await renderTemplate(
	join(localDir, 'dex-config.template.yaml'),
	join(generatedDir, 'dex-config.yaml'),
	{
		__DEX_CLIENT_ID__: localEnv.DEX_CLIENT_ID,
		__DEX_CLIENT_SECRET__: localEnv.DEX_CLIENT_SECRET,
	},
	0o644,
)

await renderTemplate(
	join(localDir, 'pomerium-config.template.yaml'),
	join(generatedDir, 'pomerium-config.yaml'),
	{
		__DEX_CLIENT_ID__: localEnv.DEX_CLIENT_ID,
		__DEX_CLIENT_SECRET__: localEnv.DEX_CLIENT_SECRET,
		__POMERIUM_COOKIE_SECRET__: localEnv.POMERIUM_COOKIE_SECRET,
		__POMERIUM_SHARED_SECRET__: localEnv.POMERIUM_SHARED_SECRET,
	},
	0o644,
)

const tlsKey = join(certDir, 'pomerium-local.key')
const tlsCert = join(certDir, 'pomerium-local.crt')
if (!existsSync(tlsKey) || !existsSync(tlsCert)) {
	execFileSync(
		'openssl',
		[
			'req',
			'-x509',
			'-newkey',
			'rsa:2048',
			'-nodes',
			'-keyout',
			tlsKey,
			'-out',
			tlsCert,
			'-days',
			'3650',
			'-subj',
			'/CN=localhost.pomerium.io',
			'-addext',
			'subjectAltName=DNS:localhost.pomerium.io,DNS:*.localhost.pomerium.io',
			'-addext',
			'basicConstraints=critical,CA:TRUE',
			'-addext',
			'keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign',
			'-addext',
			'extendedKeyUsage=serverAuth',
		],
		{ stdio: 'ignore' },
	)
	await chmod(tlsKey, 0o600)
}

const signingKey = join(keyDir, 'pomerium-signing-key.pem')
if (!existsSync(signingKey)) {
	execFileSync(
		'openssl',
		['ecparam', '-name', 'prime256v1', '-genkey', '-noout', '-out', signingKey],
		{ stdio: 'ignore' },
	)
	await chmod(signingKey, 0o600)
}

console.log(`Local Pomerium files generated in ${generatedDir}`)
console.log(`Local environment written to ${envFile}`)
console.log(`Start the stack with: ${composeUserEnv()} docker compose up --build`)
console.log('Run the smoke test with: make smoke-pomerium')
