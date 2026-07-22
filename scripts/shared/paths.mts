import { fileURLToPath } from 'node:url'
import { dirname, join, resolve } from 'node:path'

/** Repo root returns the root of the project repository. */
export const repoRoot = () => resolve(dirname(fileURLToPath(import.meta.url)), '../..')

/** Bruno test directory resolves to the test directory for the provided Bruno test collection. */
export const brunoTestDir = (collectionName: string) => join(repoRoot(), 'bruno', collectionName)

/** Local Pomerium deployment directory. */
export const localPomeriumDir = () => join(repoRoot(), 'deploy', 'local-pomerium')

/** BrunoExe resolves to the path to the Bruno executable. */
export const brunoExe = ({
	platform = process.platform,
}: {
	platform?: NodeJS.Platform
} = {}) => join(repoRoot(), 'node_modules', '.bin', platform === 'win32' ? 'bru.cmd' : 'bru')
