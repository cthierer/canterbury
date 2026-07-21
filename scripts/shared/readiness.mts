import { createConnection } from 'node:net'
import { sleep } from './processes.mts'

/** Returns a useful message for unknown caught values. */
const getErrorMessage = (error: unknown) => {
	return error instanceof Error ? error.message : String(error)
}

/** Attempts one short-lived TCP connection for readiness probing. */
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

/** Splits a host:port address while rejecting values without a host component. */
const splitHostPort = (address: string) => {
	const separator = address.lastIndexOf(':')
	if (separator < 1) {
		throw new Error(`address ${address} must be host:port`)
	}

	return [address.slice(0, separator), address.slice(separator + 1)]
}

/** Polls an HTTP endpoint until it returns a successful status or the deadline expires. */
export const waitForHTTP = async (url: string, label: string) => {
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

/** Polls a host:port listener until a TCP connection succeeds or the deadline expires. */
export const waitForTCP = async (address: string, label: string) => {
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
