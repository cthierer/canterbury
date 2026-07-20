import { mkdtemp, rm } from 'node:fs/promises'
import { createConnection } from 'node:net'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'
import type { ChildProcessByStdio } from 'node:child_process'
import type { Readable } from 'node:stream'

type ForegroundOptions = {
	cwd: string
	env: NodeJS.ProcessEnv
}

type ManagedChild = ChildProcessByStdio<null, Readable, Readable>

const root = dirname(dirname(fileURLToPath(import.meta.url)))
const devAuthAddress = process.env.DEV_AUTH_ADDR ?? '127.0.0.1:50052'
const vaultAddress = process.env.VAULT_SERVICE_ADDR ?? '127.0.0.1:50051'
const devAuthBaseURL = `http://${devAuthAddress}`
const vaultBaseURL = `http://${vaultAddress}`
const auditRoot = await mkdtemp(join(tmpdir(), 'canterbury-smoke-audit-'))
const children: ManagedChild[] = []

let shuttingDown = false
let cleanupStarted = false

const startProcess = (
	label: string,
	command: readonly [string, ...string[]],
	extraEnv: NodeJS.ProcessEnv,
) => {
	const [program, ...args] = command
	const child = spawn(program, args, {
		cwd: root,
		env: { ...process.env, ...extraEnv },
		detached: process.platform !== 'win32',
		stdio: ['ignore', 'pipe', 'pipe'],
	})

	child.stdout.on('data', chunk => {
		process.stdout.write(prefixLines(label, chunk))
	})
	child.stderr.on('data', chunk => {
		process.stderr.write(prefixLines(label, chunk))
	})

	child.once('exit', (code, signal) => {
		if (!shuttingDown) {
			process.stderr.write(
				`${label} exited unexpectedly with code ${code ?? 'null'} and signal ${signal ?? 'null'}\n`,
			)
		}
	})

	return child
}

const runForeground = (command: string, args: string[], options: ForegroundOptions) => {
	return new Promise<number>((resolve, reject) => {
		const child = spawn(command, args, {
			cwd: options.cwd,
			env: { ...process.env, ...options.env },
			stdio: 'inherit',
		})

		child.once('error', reject)
		child.once('exit', (code, signal) => {
			if (signal) {
				resolve(1)
				return
			}

			resolve(code ?? 1)
		})
	})
}

const getErrorMessage = (error: unknown) => {
	return error instanceof Error ? error.message : String(error)
}

const hasErrorCode = (error: unknown, code: string) => {
	return (
		typeof error === 'object' &&
		error !== null &&
		'code' in error &&
		(error as { code?: unknown }).code === code
	)
}

const waitForHTTP = async (url: string, label: string) => {
	const deadline = Date.now() + 20_000
	let lastError: unknown

	while (Date.now() < deadline) {
		try {
			const response = await fetch(url, { signal: AbortSignal.timeout(500) })
			if (response.ok) {
				return
			}

			lastError = new Error(`${label} returned HTTP ${response.status}`)
		} catch (error) {
			lastError = error
		}

		await sleep(250)
	}

	throw new Error(`timed out waiting for ${label}: ${getErrorMessage(lastError ?? 'no response')}`)
}

const waitForTCP = async (address: string, label: string) => {
	const [host, portString] = splitHostPort(address)
	const port = Number(portString)
	const deadline = Date.now() + 20_000
	let lastError: unknown

	while (Date.now() < deadline) {
		try {
			await connectOnce(host, port)
			return
		} catch (error) {
			lastError = error
		}

		await sleep(250)
	}

	throw new Error(`timed out waiting for ${label}: ${getErrorMessage(lastError ?? 'no listener')}`)
}

const connectOnce = (host: string, port: number) => {
	return new Promise<void>((resolve, reject) => {
		const socket = createConnection({ host, port })
		socket.setTimeout(500)
		socket.once('connect', () => {
			socket.end()
			resolve()
		})
		socket.once('timeout', () => {
			socket.destroy(new Error('connection timed out'))
		})
		socket.once('error', reject)
	})
}

const stopChildren = async () => {
	shuttingDown = true
	await Promise.all([...children].reverse().map(child => stopChild(child)))
}

const cleanup = async () => {
	if (cleanupStarted) {
		return
	}

	cleanupStarted = true
	await stopChildren()
	await rm(auditRoot, { recursive: true, force: true })
}

const stopChild = (child: ManagedChild) => {
	return new Promise<void>(resolve => {
		if (child.exitCode !== null || child.signalCode !== null) {
			resolve()
			return
		}

		const timeout = setTimeout(() => {
			killChild(child, 'SIGKILL')
			resolve()
		}, 5_000)

		child.once('exit', () => {
			clearTimeout(timeout)
			resolve()
		})

		killChild(child, 'SIGTERM')
	})
}

const killChild = (child: ManagedChild, signal: NodeJS.Signals) => {
	try {
		if (process.platform === 'win32') {
			child.kill(signal)
			return
		}

		if (child.pid === undefined) {
			return
		}

		process.kill(-child.pid, signal)
	} catch (error) {
		if (!hasErrorCode(error, 'ESRCH')) {
			throw error
		}
	}
}

const splitHostPort = (address: string) => {
	const separator = address.lastIndexOf(':')
	if (separator < 1) {
		throw new Error(`address ${address} must be host:port`)
	}

	return [address.slice(0, separator), address.slice(separator + 1)]
}

const prefixLines = (label: string, chunk: Buffer) => {
	return chunk
		.toString()
		.split('\n')
		.map((line, index, lines) => {
			if (index === lines.length - 1 && line === '') {
				return line
			}

			return `[${label}] ${line}`
		})
		.join('\n')
}

const sleep = (ms: number) => {
	return new Promise<void>(resolve => {
		setTimeout(resolve, ms)
	})
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
	const devAuth = startProcess('dev-auth', ['go', 'run', './cmd/dev-auth', 'serve'], {
		DEV_AUTH_ADDR: devAuthAddress,
		DEV_AUTH_ISSUER: 'devauth.canterbury.local',
	})
	children.push(devAuth)

	await waitForHTTP(`${devAuthBaseURL}/.well-known/jwks.json`, 'development auth JWKS')

	const vault = startProcess('vault-service', ['go', 'run', './cmd/vault-service'], {
		VAULT_SERVICE_ADDR: vaultAddress,
		VAULT_SERVICE_ROOT: './sample-vault',
		VAULT_SERVICE_AUTH_ISSUER: 'devauth.canterbury.local',
		VAULT_SERVICE_AUTH_AUDIENCE: 'canterbury.vault.local',
		VAULT_SERVICE_AUTH_JWKS_URL: `${devAuthBaseURL}/.well-known/jwks.json`,
		VAULT_SERVICE_AUTH_MAPPING_FILE: './sample-auth/scopes.toml',
		VAULT_SERVICE_AUDIT_ROOT: auditRoot,
		VAULT_SERVICE_AUDIT_WRITER_ID: 'local-auth-smoke',
		VAULT_SERVICE_AUDIT_HMAC_KEY: 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=',
	})
	children.push(vault)

	await waitForTCP(vaultAddress, 'vault service')

	const bruBin = join(
		root,
		'node_modules',
		'.bin',
		process.platform === 'win32' ? 'bru.cmd' : 'bru',
	)
	const result = await runForeground(
		bruBin,
		[
			'run',
			'--env',
			'local',
			'--env-var',
			`DevAuthBaseURI=${devAuthBaseURL}`,
			'--env-var',
			`VaultBaseURI=${vaultBaseURL}`,
			'--bail',
			'--noproxy',
		],
		{
			cwd: join(root, 'bruno', 'local-auth-smoke'),
			env: {
				NO_PROXY: '127.0.0.1,localhost',
				no_proxy: '127.0.0.1,localhost',
			},
		},
	)

	process.exitCode = result
} finally {
	await cleanup()
}
