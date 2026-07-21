/** Parsed HTTP response envelope returned by script-local clients. */
export interface HTTPResponse {
	readonly status: number
	readonly body: unknown
}

/** Parses a response body as JSON, preserving raw text for non-JSON error bodies. */
export const parseJSON = async (response: Response): Promise<unknown> => {
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
