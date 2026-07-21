import { parseJSON, type HTTPResponse } from './http.mts'

/** Minimal JSON-RPC MCP client for stateless HTTP smoke tests. */
export class MCPClient {
	readonly baseURL: string

	private requestID = 0

	constructor(baseURL: string) {
		this.baseURL = baseURL
	}

	/** Posts one JSON-RPC request to the MCP endpoint. */
	async post(method: string, params: unknown, token?: string): Promise<HTTPResponse> {
		const headers: Record<string, string> = {
			accept: 'application/json, text/event-stream',
			'content-type': 'application/json',
			'mcp-protocol-version': '2025-06-18',
		}

		if (token) {
			headers.authorization = `Bearer ${token}`
		}

		const response = await fetch(`${this.baseURL}/mcp`, {
			method: 'POST',
			headers,
			body: JSON.stringify({
				jsonrpc: '2.0',
				id: ++this.requestID,
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
}
