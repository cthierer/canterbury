#!/usr/bin/env node

/*
 * Container entrypoint for the Obsidian Sync worker.
 *
 * The script bridges Docker-provided configuration into obsidian-headless:
 * it writes the auth token where the `ob` CLI expects it, ensures the vault is
 * configured once, then hands off to `ob sync --continuous`. It also forwards
 * shutdown signals so Docker stops release the Obsidian sync lock cleanly.
 */

import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import { homedir } from 'node:os'
import { join, dirname } from 'node:path'
import dotenv from 'dotenv'

const EXIT_MISSING_VAULT_NAME = 10
const EXIT_MISSING_VAULT_PASSWORD = 11
const EXIT_MISSING_OBSIDIAN_AUTH_TOKEN = 12

const moduleDirname = import.meta.dirname
const obPath = join(moduleDirname, 'node_modules/.bin/ob')
const activeChildren = new Set()
const signalExitCodes = {
	SIGINT: 130,
	SIGTERM: 143,
}

let shutdownSignal = null

dotenv.config({
	path: join(moduleDirname, '.env'),
	quiet: true,
})

const {
	SYNC_VAULT_PATH: vaultPath = join(process.cwd(), './vault'),
	SYNC_VAULT_NAME: vaultName = '',
	SYNC_VAULT_PASSWORD: vaultPassword = '',
	SYNC_OBSIDIAN_AUTH_TOKEN: obsidianAuthToken = '',
	SYNC_DEVICE_NAME: deviceName = 'canterbury-sync',
} = process.env

const camelToKebab = value => value.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`)

const obArgs = mapping =>
	Object.entries(mapping).flatMap(([name, value]) => [`--${camelToKebab(name)}`, value])

const redactArgs = args =>
	args.map((arg, index) => {
		if (args[index - 1] === '--password') {
			return '[redacted]'
		}

		return arg
	})

const describeCommand = (command, args = []) => [command, ...redactArgs(args)].join(' ')

const createSignalExit = () => {
	const error = new Error(`Received ${shutdownSignal}`)
	error.exitCode = signalExitCodes[shutdownSignal] ?? 1
	error.silent = true
	return error
}

const createConfigError = (message, exitCode) => {
	const error = new Error(message)
	error.exitCode = exitCode
	return error
}

const throwIfShuttingDown = () => {
	if (shutdownSignal) {
		throw createSignalExit()
	}
}

const forwardSignal = signal => {
	shutdownSignal = signal

	for (const child of activeChildren) {
		child.kill(signal)
	}
}

process.once('SIGINT', () => forwardSignal('SIGINT'))
process.once('SIGTERM', () => forwardSignal('SIGTERM'))

const validateEnvironment = () => {
	if (!vaultName) {
		throw createConfigError('SYNC_VAULT_NAME is required', EXIT_MISSING_VAULT_NAME)
	}

	if (!vaultPassword) {
		throw createConfigError('SYNC_VAULT_PASSWORD is required', EXIT_MISSING_VAULT_PASSWORD)
	}

	if (!obsidianAuthToken) {
		throw createConfigError(
			'SYNC_OBSIDIAN_AUTH_TOKEN is required',
			EXIT_MISSING_OBSIDIAN_AUTH_TOKEN,
		)
	}
}

const spawnAsync = (command, args = [], options = {}) =>
	new Promise((resolve, reject) => {
		const { rejectOnError = true, label = command, ...spawnOptions } = options
		const child = spawn(command, args, spawnOptions)
		const commandDescription = describeCommand(label, args)

		let settled = false

		activeChildren.add(child)

		child.on('error', error => {
			activeChildren.delete(child)

			if (settled) {
				return
			}

			settled = true
			reject(new Error(`Failed to start ${commandDescription}: ${error.message}`))
		})

		child.on('close', (code, signal) => {
			activeChildren.delete(child)

			if (settled) {
				return
			}

			settled = true

			if (code === 0 || !rejectOnError || shutdownSignal) {
				resolve({ code, signal })
				return
			}

			reject(new Error(`${commandDescription} exited with code ${code ?? signal}`))
		})
	})

const runObCommand = async (command, commandArgs = [], options = {}) => {
	throwIfShuttingDown()

	const result = await spawnAsync(obPath, [command, ...commandArgs], {
		label: 'ob',
		rejectOnError: options.rejectOnError,
		cwd: moduleDirname,
		stdio: options.stdio ?? 'inherit',
		env: process.env,
	})

	throwIfShuttingDown()
	return result
}

const writeAuthToken = async () => {
	const authTokenPath = join(homedir(), '/.config/obsidian-headless/auth_token')

	await mkdir(dirname(authTokenPath), { recursive: true })
	await writeFile(authTokenPath, obsidianAuthToken, { mode: 0o600 })
}

const setupSync = () =>
	runObCommand(
		'sync-setup',
		obArgs({
			vault: vaultName,
			path: vaultPath,
			password: vaultPassword,
			deviceName,
		}),
	)

const setupSyncIfNeeded = async () => {
	const status = await runObCommand(
		'sync-status',
		obArgs({
			path: vaultPath,
		}),
		{
			rejectOnError: false,
		},
	)

	if (status.code === 0) {
		return
	}

	if (status.code === 3) {
		await setupSync()
		return
	}

	throw new Error(`ob sync-status exited with code ${status.code ?? status.signal}`)
}

const runContinuousSync = () =>
	runObCommand('sync', [
		...obArgs({
			path: vaultPath,
		}),
		'--continuous',
	])

const main = async () => {
	validateEnvironment()
	await writeAuthToken()
	await setupSyncIfNeeded()
	await runContinuousSync()
}

try {
	await main()
} catch (error) {
	if (!error.silent) {
		console.error(error.message)
	}

	process.exit(error.exitCode ?? 1)
}
