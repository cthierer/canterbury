import { brunoExe, brunoTestDir } from './paths.mts'
import { runForeground, type ProcessExit } from './processes.mts'

export interface BrunoPhaseOptions {
	readonly collection: string
	readonly envName: string
	readonly envVars?: Record<string, string>
	readonly noProxy?: string
}

type RunForeground = typeof runForeground

/** Builds the Bruno CLI command for one tagged phase. */
export const brunoPhaseCommand = (tag: string, options: BrunoPhaseOptions) => {
	const command: [string, ...string[]] = [brunoExe(), 'run', '--env', options.envName]
	for (const [key, value] of Object.entries(options.envVars ?? {})) {
		command.push('--env-var', `${key}=${value}`)
	}

	command.push('--tags', tag, '--bail', '--noproxy')
	return command
}

/** Runs one tagged Bruno phase and fails when Bruno exits unsuccessfully. */
export const runBrunoPhase = async (
	tag: string,
	options: BrunoPhaseOptions,
	{ runner = runForeground }: { runner?: RunForeground } = {},
) => {
	const result: ProcessExit = await runner(brunoPhaseCommand(tag, options), {
		cwd: brunoTestDir(options.collection),
		env: {
			...process.env,
			NO_PROXY: options.noProxy ?? '127.0.0.1,localhost',
			no_proxy: options.noProxy ?? '127.0.0.1,localhost',
		},
	})

	if (result.signal) {
		throw new Error(`Bruno phase ${tag} exited with signal ${result.signal}`)
	}

	if (result.code !== 0) {
		throw new Error(`Bruno phase ${tag} exited with code ${result.code ?? 'null'}`)
	}
}
