import { readFile } from 'node:fs/promises'
import { getErrorMessage, hasErrorCode } from './errors.mts'

/** Environment-file values parsed from KEY=value lines. */
export type EnvValues = Record<string, string>

/** Parses simple KEY=value environment-file content. */
export const parseEnvValues = (data: string): EnvValues => {
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

/** Reads an optional environment file without treating missing files as errors. */
export const readEnvFile = async (path: string): Promise<EnvValues> => {
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

	return parseEnvValues(data)
}

/** Loads environment-file values only when the target environment has not already provided them. */
export const loadEnvFile = async (
	path: string,
	{ env = process.env }: { env?: NodeJS.ProcessEnv } = {},
) => {
	const values = await readEnvFile(path)
	for (const [key, value] of Object.entries(values)) {
		if (env[key] === undefined) {
			env[key] = value
		}
	}
}
