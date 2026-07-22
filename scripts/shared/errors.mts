/** Checks Node-style thrown values for a specific filesystem or process error code. */
export const hasErrorCode = (error: unknown, code: string) =>
	typeof error === 'object' &&
	error !== null &&
	'code' in error &&
	(error as { code?: unknown }).code === code

/** Returns a useful message for unknown caught values. */
export const getErrorMessage = (error: unknown) =>
	error instanceof Error ? error.message : String(error)

/** Narrows parsed JSON to an object before reading nested response fields. */
export const isRecord = (value: unknown): value is Record<string, unknown> =>
	typeof value === 'object' && value !== null
