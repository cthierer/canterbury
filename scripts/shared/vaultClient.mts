import { parseJSON, type HTTPResponse } from './http.mts'

/** Minimal Connect JSON request body for VaultService.ReadNote. */
export interface ReadNoteBody {
	readonly ref: {
		readonly path: string
	}
}

/** Builds the request body for VaultService.ReadNote. */
export const readNoteBody = (path: string): ReadNoteBody => {
	return {
		ref: {
			path,
		},
	}
}

/** Posts a Connect JSON request to the vault service. */
export const postVault = async (
	baseURL: string,
	method: string,
	body: unknown,
	token?: string,
): Promise<HTTPResponse> => {
	const headers: Record<string, string> = {
		'content-type': 'application/json',
	}

	if (token) {
		headers.authorization = `Bearer ${token}`
	}

	const response = await fetch(`${baseURL}/canterbury.vault.v1.VaultService/${method}`, {
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
