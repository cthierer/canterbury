#!/usr/bin/env node

import { pathToFileURL } from 'node:url'
import dotenv from 'dotenv'
import { readHealth } from './health.js'
import { loadWorkerConfig, runWorker } from './worker.js'

const moduleDirname = import.meta.dirname

const loadEnvironment = () =>
	dotenv.config({
		path: `${moduleDirname}/.env`,
		quiet: true,
	})

const parseHealthcheckMode = args => {
	let mode = 'ready'

	for (let index = 0; index < args.length; index += 1) {
		if (args[index] !== '--mode' || index + 1 >= args.length) {
			throw new Error(`Unexpected healthcheck argument ${args[index] ?? ''}`.trim())
		}

		mode = args[index + 1]
		index += 1
	}

	if (mode !== 'live' && mode !== 'ready') {
		throw new Error('Healthcheck mode must be live or ready')
	}

	return mode
}

export const run = async ({ args = process.argv.slice(2), output = process.stdout } = {}) => {
	loadEnvironment()
	const config = loadWorkerConfig({ moduleDirname })

	if (args[0] === 'healthcheck') {
		let mode
		try {
			mode = parseHealthcheckMode(args.slice(1))
		} catch {
			output.write('NOT_READY\n')
			return 1
		}

		const healthy = await readHealth({ statePath: config.statePath, mode })
		output.write(`${healthy ? mode.toUpperCase() : `NOT_${mode.toUpperCase()}`}\n`)
		return healthy ? 0 : 1
	}

	if (args.length > 0) {
		console.error(`Unexpected command ${args[0]}`)
		return 1
	}

	try {
		await runWorker({ config, moduleDirname })
		return 0
	} catch (error) {
		if (!error.silent) {
			console.error(error.message)
		}
		return error.exitCode ?? 1
	}
}

const isMain = process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href

if (isMain) {
	process.exitCode = await run()
}
