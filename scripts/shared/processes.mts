import { spawn, type ChildProcessByStdio } from 'node:child_process'
import type { Readable } from 'node:stream'

/** Process exit tracks how a process was exited. */
export type ProcessExit = { code: number | null; signal: NodeJS.Signals | null }

/** Managed process represents a process under management by the process group. */
export interface ManagedProcess {
	readonly label: string
	readonly child: ChildProcessByStdio<null, Readable, Readable>
	readonly exit: Promise<ProcessExit>
}

/** Prefixes lines with the given label. */
const prefixLines = (label: string, chunk: Buffer) =>
	chunk
		.toString()
		.split('\n')
		.map((line, index, lines) => {
			if (index === lines.length - 1 && line === '') {
				return line
			}

			return `[${label}] ${line}`
		})
		.join('\n')

/** Checks Node-style thrown values for a specific filesystem or process error code. */
const hasErrorCode = (error: unknown, code: string) =>
	typeof error === 'object' &&
	error !== null &&
	'code' in error &&
	(error as { code?: unknown }).code === code

/** Waits for a small polling interval. */
export const sleep = (ms: number) =>
	new Promise<void>(resolve => {
		setTimeout(resolve, ms)
	})

export interface ProcessOptions {
	cwd?: string
	env?: NodeJS.ProcessEnv
	platform?: NodeJS.Platform
	stderr?: NodeJS.WriteStream
	stdout?: NodeJS.WriteStream
}

/** Process group tracks and manages multiple child processes as a single unit. */
export class ProcessGroup {
	/** CWD tracks the current working directory for processes spawned in this group. */
	readonly cwd: string

	/** Shared environment variables for all processes spawned in this group. */
	readonly env: NodeJS.ProcessEnv

	/** Platform these child processes are running on. */
	readonly platform: NodeJS.Platform

	/** Error stream for all processes spawned in this group. */
	readonly stderr: NodeJS.WriteStream

	/** Output stream for all processes spawned in this group. */
	readonly stdout: NodeJS.WriteStream

	/** Processes are the processes spawned in this group. */
	private processes: ManagedProcess[] = []

	constructor({
		cwd = process.cwd(),
		env = process.env,
		platform = process.platform,
		stderr = process.stderr,
		stdout = process.stdout,
	}: ProcessOptions = {}) {
		this.cwd = cwd
		this.env = env
		this.platform = platform
		this.stderr = stderr
		this.stdout = stdout
	}

	/** Detached reflects if child processes should run in detached mode. */
	private get detached(): boolean {
		return this.platform !== 'win32'
	}

	/** Spawn a new child process managed as part of this group. */
	spawn(label: string, command: readonly [string, ...string[]], extraEnv: NodeJS.ProcessEnv) {
		const [program, ...args] = command
		const child = spawn(program, args, {
			cwd: this.cwd,
			env: { ...this.env, ...extraEnv },
			detached: this.detached,
			stdio: ['ignore', 'pipe', 'pipe'],
		})

		child.stdout.on('data', chunk => {
			this.stdout.write(prefixLines(label, chunk))
		})

		child.stderr.on('data', chunk => {
			this.stderr.write(prefixLines(label, chunk))
		})

		const exit = new Promise<ProcessExit>((resolve, reject) => {
			child.once('exit', (code, signal) => {
				resolve({ code, signal })
			})

			child.once('error', err => {
				reject(err)
			})
		})

		const managedProcess = {
			label,
			child,
			exit,
		}

		this.processes.push(managedProcess)

		return managedProcess
	}

	/** Shutdown all child processes part of this group. */
	async shutdown() {
		const processes = [...this.processes].reverse()
		const errors = []

		for (const managedProcess of processes) {
			try {
				await this.stop(managedProcess)
			} catch (err) {
				errors.push(err)
			}
		}

		if (errors.length > 0) {
			throw new AggregateError(errors, `failed to stop ${errors.length} processes`)
		}

		this.processes = this.processes.filter(managedProcess => !processes.includes(managedProcess))
	}

	/** Stop one process by attempting to do it gracefully, then forcing termination. */
	private async stop(
		managedProcess: ManagedProcess,
		{ timeoutMillis = 5000 }: { timeoutMillis?: number } = {},
	) {
		if (managedProcess.child.exitCode !== null || managedProcess.child.signalCode !== null) {
			return
		}

		this.kill(managedProcess.child, 'SIGTERM')

		let timeout: NodeJS.Timeout | undefined
		const timeoutReached = new Promise<'timeout'>(resolve => {
			timeout = setTimeout(() => {
				resolve('timeout')
			}, timeoutMillis)
		})
		let exited
		try {
			exited = await Promise.race([managedProcess.exit, timeoutReached])
		} finally {
			if (timeout) {
				clearTimeout(timeout)
			}
		}

		if (exited === 'timeout') {
			this.kill(managedProcess.child, 'SIGKILL')
			await managedProcess.exit
		}
	}

	/** Kill a specific process that is part of this group. */
	private kill(child: ChildProcessByStdio<null, Readable, Readable>, signal: NodeJS.Signals) {
		try {
			if (this.platform === 'win32') {
				child.kill(signal)
				return
			}

			if (child.pid === undefined) {
				return
			}

			process.kill(-child.pid, signal)
		} catch (err) {
			if (!hasErrorCode(err, 'ESRCH')) {
				throw err
			}
		}
	}
}

/** Run a process in the foreground. */
export const runForeground = (
	command: readonly [string, ...string[]],
	{
		cwd = process.cwd(),
		env = process.env,
	}: Omit<ProcessOptions, 'platform' | 'stderr' | 'stdout'> = {},
) => {
	const [program, ...args] = command
	return new Promise<ProcessExit>((resolve, reject) => {
		const child = spawn(program, args, {
			cwd,
			env,
			stdio: 'inherit',
		})

		child.once('error', err => {
			reject(err)
		})

		child.once('exit', (code, signal) => {
			resolve({ code, signal })
		})
	})
}
