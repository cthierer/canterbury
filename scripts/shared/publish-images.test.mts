import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'

import {
	assertCleanTree,
	assertRefInput,
	isReleaseTag,
	parseBuildMetadata,
	parseImageSelection,
	parsePublishMode,
	publishImages,
	redactCommand,
	resolvePublishPlan,
	tagsForRevision,
	versionForTags,
} from './publish-images.mts'

const revision = '0123456789abcdef0123456789abcdef01234567'

test('accepts exact SemVer release tags, including prereleases', () => {
	assert.equal(isReleaseTag('v1.2.3'), true)
	assert.equal(isReleaseTag('v1.2.3-rc.1'), true)
	assert.equal(isReleaseTag('v0.0.0-alpha'), true)
	assert.equal(isReleaseTag('v1.2.3+build.1'), false)
	assert.equal(isReleaseTag('v1.2'), false)
})

test('rejects empty, moving, and build-metadata refs', () => {
	for (const ref of [undefined, '', 'latest', 'v1', 'v1.2', 'v1.2.3+build.1', 'refs/tags/latest']) {
		assert.throws(() => assertRefInput(ref))
	}
	assert.equal(assertRefInput('main'), 'main')
	assert.equal(assertRefInput('v1.2.3-rc.1'), 'v1.2.3-rc.1')
})

test('selects supported images and defaults to all', () => {
	assert.deepEqual(parseImageSelection(undefined), ['mcp-server', 'vault-service', 'sync'])
	assert.deepEqual(parseImageSelection('vault-service'), ['vault-service'])
	assert.throws(() => parseImageSelection('worker'))
})

test('selects explicit publish modes and defaults safely', () => {
	assert.equal(parsePublishMode(undefined), 'dry-run')
	assert.equal(parsePublishMode('build'), 'build')
	assert.equal(parsePublishMode('push'), 'push')
	assert.throws(() => parsePublishMode('publish'))
})

test('creates immutable SHA tags and exact release tags only', () => {
	assert.deepEqual(tagsForRevision(revision, []), [`sha-${revision}`])
	assert.deepEqual(tagsForRevision(revision, ['v1.2.3']), [`sha-${revision}`, 'v1.2.3'])
	assert.deepEqual(tagsForRevision(revision, ['latest', 'v1']), [`sha-${revision}`])
	assert.throws(() => tagsForRevision(revision, ['v1.2.3', 'v1.2.4']))
})

test('uses an exact release tag as the OCI and runtime version when available', () => {
	assert.equal(versionForTags([`sha-${revision}`]), `sha-${revision}`)
	assert.equal(versionForTags([`sha-${revision}`, 'v1.2.3-rc.1']), 'v1.2.3-rc.1')
	assert.throws(() => versionForTags([]))
})

test('requires a clean tree for pushes', () => {
	assert.doesNotThrow(() => assertCleanTree(''))
	assert.throws(() => assertCleanTree(' M README.md'))
})

test('resolves only the checked-out revision and reads its commit time', () => {
	const calls: string[] = []
	const run = (command: string, args: string[]): string => {
		calls.push(`${command} ${args.join(' ')}`)
		if (args[0] === 'rev-parse') return revision
		if (args[0] === 'status') return ''
		if (args[0] === 'tag') return 'v1.2.3-rc.1'
		if (args.join(' ') === `show -s --format=%cI ${revision}`) {
			return '2026-08-06T12:34:56+00:00'
		}
		if (args.join(' ') === `show -s --format=%ct ${revision}`) return '1786019696'
		throw new Error(`unexpected command: ${args.join(' ')}`)
	}

	const plan = resolvePublishPlan({ image: 'sync', mode: 'push', ref: 'v1.2.3-rc.1' }, run)
	assert.equal(plan.revision, revision)
	assert.equal(plan.sourceDateEpoch, '1786019696')
	assert.deepEqual(plan.tags, [`sha-${revision}`, 'v1.2.3-rc.1'])
	assert.equal(calls.includes('git status --porcelain'), true)
})

test('rejects refs that do not resolve to checked-out HEAD', () => {
	const run = (_command: string, args: string[]): string =>
		args.includes('HEAD^{commit}') ? revision : 'fedcba9876543210fedcba9876543210fedcba98'
	assert.throws(() => resolvePublishPlan({ image: 'all', mode: 'build', ref: 'main' }, run))
})

test('rejects a release ref that is not an actual tag at the revision', () => {
	const run = (_command: string, args: string[]): string => {
		if (args[0] === 'rev-parse') return revision
		if (args[0] === 'tag') return ''
		if (args.join(' ') === `show -s --format=%cI ${revision}`) {
			return '2026-08-06T12:34:56+00:00'
		}
		if (args.join(' ') === `show -s --format=%ct ${revision}`) return '1786019696'
		throw new Error(`unexpected command: ${args.join(' ')}`)
	}

	assert.throws(() => resolvePublishPlan({ image: 'all', mode: 'build', ref: 'v1.2.3' }, run))
})

test('rejects an invalid commit timestamp epoch', () => {
	const run = (_command: string, args: string[]): string => {
		if (args[0] === 'rev-parse') return revision
		if (args[0] === 'tag') return ''
		if (args.join(' ') === `show -s --format=%cI ${revision}`) {
			return '2026-08-06T12:34:56+00:00'
		}
		if (args.join(' ') === `show -s --format=%ct ${revision}`) return 'not-an-epoch'
		throw new Error(`unexpected command: ${args.join(' ')}`)
	}

	assert.throws(() => resolvePublishPlan({ image: 'all', mode: 'build', ref: 'HEAD' }, run))
})

test('parses the selected target digest from Buildx metadata', () => {
	const digest = `sha256:${'a'.repeat(64)}`
	const metadata = JSON.stringify({
		'mcp-server': {
			'buildx.build.ref': 'default/default/example',
			'containerimage.digest': digest,
		},
	})
	assert.equal(parseBuildMetadata(metadata, 'mcp-server'), digest)
	assert.throws(() => parseBuildMetadata(metadata, 'sync'))
	assert.throws(() => parseBuildMetadata('{}', 'mcp-server'))
	assert.throws(() =>
		parseBuildMetadata(JSON.stringify({ 'containerimage.digest': digest }), 'mcp-server'),
	)
	assert.throws(() =>
		parseBuildMetadata(
			JSON.stringify({ 'mcp-server': { 'containerimage.digest': 'sha256:not-a-digest' } }),
			'mcp-server',
		),
	)
})

test('uses release versions in Bake arguments and emits resolved GitHub outputs', () => {
	const directory = mkdtempSync(join(tmpdir(), 'canterbury-publish-images-'))
	const digestFile = join(directory, 'image-digests.json')
	const metadataFile = `${digestFile}.mcp-server.metadata.json`
	const githubOutput = join(directory, 'github-output')
	const digest = `sha256:${'b'.repeat(64)}`
	const previousGitHubOutput = process.env.GITHUB_OUTPUT
	const calls: string[][] = []

	try {
		writeFileSync(
			metadataFile,
			JSON.stringify({ 'mcp-server': { 'containerimage.digest': digest } }),
		)
		process.env.GITHUB_OUTPUT = githubOutput

		publishImages(
			{
				created: '2026-08-06T12:34:56+00:00',
				digestFile,
				images: ['mcp-server'],
				mode: 'push',
				revision,
				sourceDateEpoch: '1786019696',
				tags: [`sha-${revision}`, 'v1.2.3'],
			},
			(_command, args) => {
				calls.push(args)
				return ''
			},
		)

		assert.equal(calls[0].includes('mcp-server.args.CANTERBURY_VERSION=v1.2.3'), true)
		assert.equal(
			calls[0].includes('mcp-server.labels.org.opencontainers.image.version=v1.2.3'),
			true,
		)
		assert.equal(calls[0].includes('mcp-server.args.SOURCE_DATE_EPOCH=1786019696'), true)
		assert.equal(
			calls[0].includes('mcp-server.output=type=image,push=true,rewrite-timestamp=true'),
			true,
		)
		assert.deepEqual(JSON.parse(readFileSync(digestFile, 'utf8')), { 'mcp-server': digest })
		assert.equal(
			readFileSync(githubOutput, 'utf8'),
			`revision=${revision}\nmcp_server_digest=${digest}\n`,
		)
	} finally {
		if (previousGitHubOutput === undefined) {
			delete process.env.GITHUB_OUTPUT
		} else {
			process.env.GITHUB_OUTPUT = previousGitHubOutput
		}
		rmSync(directory, { force: true, recursive: true })
	}
})

test('redacts credential-like command arguments before display', () => {
	assert.equal(
		redactCommand('docker login --password=very-secret token=abc authorization=Bearer'),
		'docker login --password=[REDACTED] token=[REDACTED] authorization=[REDACTED]',
	)
})
