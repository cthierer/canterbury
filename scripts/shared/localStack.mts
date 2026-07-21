import type { ProcessExit, ProcessGroup } from './processes.mts'
import { waitForHTTP, waitForTCP } from './readiness.mts'

const formatExit = ({ code, signal }: ProcessExit) =>
	`code=${code ?? 'null'}, signal=${signal ?? 'null'}`

export interface DevAuthService {
	readonly baseURL: string
	readonly jwksURL: string
}

export const startDevAuth = async (
	processGroup: ProcessGroup,
	devAuthAddress: string,
): Promise<DevAuthService> => {
	const devAuth = processGroup.spawn('dev-auth', ['go', 'run', './cmd/dev-auth', 'serve'], {
		DEV_AUTH_ADDR: devAuthAddress,
		DEV_AUTH_ISSUER: 'devauth.canterbury.local',
	})

	const baseURL = `http://${devAuthAddress}`
	const jwksURL = `${baseURL}/.well-known/jwks.json`

	await Promise.race([
		waitForHTTP(jwksURL, 'development auth JWKS'),
		devAuth.exit.then(exit => {
			throw new Error(`dev-auth exited before readiness: ${formatExit(exit)}`)
		}),
	])

	return {
		baseURL,
		jwksURL,
	}
}

export interface VaultService {
	readonly baseURL: string
}

export const startVaultService = async (
	processGroup: ProcessGroup,
	vaultAddress: string,
	jwksAddress: string,
	auditRoot: string,
): Promise<VaultService> => {
	const vaultService = processGroup.spawn('vault-service', ['go', 'run', './cmd/vault-service'], {
		VAULT_SERVICE_ADDR: vaultAddress,
		VAULT_SERVICE_ROOT: './sample-vault',
		VAULT_SERVICE_AUTH_ISSUER: 'devauth.canterbury.local',
		VAULT_SERVICE_AUTH_AUDIENCE: 'canterbury.vault.local',
		VAULT_SERVICE_AUTH_JWKS_URL: jwksAddress,
		VAULT_SERVICE_AUTH_MAPPING_FILE: './sample-auth/scopes.toml',
		VAULT_SERVICE_AUDIT_ROOT: auditRoot,
		VAULT_SERVICE_AUDIT_WRITER_ID: 'local-auth-smoke',
		VAULT_SERVICE_AUDIT_HMAC_KEY: 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=',
	})

	await Promise.race([
		waitForTCP(vaultAddress, 'vault service'),
		vaultService.exit.then(exit => {
			throw new Error(`vault-service exited before ready: ${formatExit(exit)}`)
		}),
	])

	const baseURL = `http://${vaultAddress}`
	return { baseURL }
}
