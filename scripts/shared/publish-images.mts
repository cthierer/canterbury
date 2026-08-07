import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname } from 'node:path'

export const imageNames = {
	'mcp-server': 'ghcr.io/cthierer/canterbury-mcp-server',
	'vault-service': 'ghcr.io/cthierer/canterbury-vault-service',
	sync: 'ghcr.io/cthierer/canterbury-sync',
} as const

export type ImageName = keyof typeof imageNames
export type PublishMode = 'dry-run' | 'build' | 'push'

export type PublishPlan = {
	created: string
	digestFile: string
	images: ImageName[]
	mode: PublishMode
	revision: string
	tags: string[]
}

type CommandRunner = (command: string, args: string[]) => string

const releaseTagPattern =
	/^v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-(?:(?:0|[1-9]\d*)|(?:[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))(?:\.(?:(?:0|[1-9]\d*)|(?:[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)))*)?$/

const forbiddenTagPattern = /^(?:latest|v?\d+(?:\.\d+)?)(?:$|[+])/i

const runCommand: CommandRunner = (command, args) =>
	execFileSync(command, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()

export const parseImageSelection = (value: string | undefined): ImageName[] => {
	if (value === undefined || value === '' || value === 'all') {
		return Object.keys(imageNames) as ImageName[]
	}

	if (Object.hasOwn(imageNames, value)) {
		return [value as ImageName]
	}

	throw new Error(
		`IMAGE must be all, mcp-server, vault-service, or sync; received ${JSON.stringify(value)}`,
	)
}

export const parsePublishMode = (value: string | undefined): PublishMode => {
	if (value === undefined || value === '') {
		return 'dry-run'
	}

	if (value === 'dry-run' || value === 'build' || value === 'push') {
		return value
	}

	throw new Error(`MODE must be dry-run, build, or push; received ${JSON.stringify(value)}`)
}

export const assertRefInput = (ref: string | undefined): string => {
	if (ref === undefined || ref.trim() === '') {
		throw new Error('REF is required and must name one local Git revision')
	}

	if (ref !== ref.trim() || /\s/.test(ref)) {
		throw new Error('REF must not contain whitespace')
	}

	const tag = ref.replace(/^refs\/tags\//, '')
	if (tag === 'latest' || forbiddenTagPattern.test(tag)) {
		throw new Error(`REF ${JSON.stringify(ref)} is a forbidden or moving image tag`)
	}

	if ((ref.startsWith('refs/tags/') || ref.startsWith('v')) && !releaseTagPattern.test(tag)) {
		throw new Error(`REF ${JSON.stringify(ref)} is not an exact supported release tag`)
	}

	return ref
}

export const isReleaseTag = (tag: string): boolean => releaseTagPattern.test(tag)

export const tagsForRevision = (revision: string, tagsAtRevision: string[]): string[] => {
	const releaseTags = tagsAtRevision.filter(isReleaseTag)
	if (releaseTags.length > 1) {
		throw new Error(`revision ${revision} has ambiguous release tags: ${releaseTags.join(', ')}`)
	}

	return [`sha-${revision}`, ...releaseTags]
}

export const assertCleanTree = (status: string): void => {
	if (status !== '') {
		throw new Error('MODE=push requires a clean working tree')
	}
}

export const resolvePublishPlan = (
	{
		ref,
		image,
		mode,
		digestFile = '.cache/image-digests.json',
	}: {
		ref: string | undefined
		image: string | undefined
		mode: string | undefined
		digestFile?: string
	},
	run: CommandRunner = runCommand,
): PublishPlan => {
	const requestedRef = assertRefInput(ref)
	const selectedMode = parsePublishMode(mode)
	const revision = run('git', ['rev-parse', '--verify', `${requestedRef}^{commit}`])
	const head = run('git', ['rev-parse', '--verify', 'HEAD^{commit}'])

	if (!/^[0-9a-f]{40}$/.test(revision) || !/^[0-9a-f]{40}$/.test(head)) {
		throw new Error('Git must resolve REF and HEAD to full 40-character commit IDs')
	}

	if (revision !== head) {
		throw new Error(`REF resolves to ${revision}, but checked-out HEAD is ${head}`)
	}

	if (selectedMode === 'push') {
		assertCleanTree(run('git', ['status', '--porcelain']))
	}

	const tagsAtRevision = run('git', ['tag', '--points-at', revision]).split('\n').filter(Boolean)
	const requestedTag = requestedRef.replace(/^refs\/tags\//, '')
	if (isReleaseTag(requestedTag) && !tagsAtRevision.includes(requestedTag)) {
		throw new Error(`release REF ${JSON.stringify(requestedRef)} is not a tag at ${revision}`)
	}
	const tags = tagsForRevision(revision, tagsAtRevision)
	const created = run('git', ['show', '-s', '--format=%cI', revision])

	if (Number.isNaN(Date.parse(created))) {
		throw new Error(`Git returned an invalid commit timestamp: ${JSON.stringify(created)}`)
	}

	return {
		created,
		digestFile,
		images: parseImageSelection(image),
		mode: selectedMode,
		revision,
		tags,
	}
}

export const parseBuildMetadata = (contents: string): string => {
	const metadata: unknown = JSON.parse(contents)
	if (typeof metadata !== 'object' || metadata === null || Array.isArray(metadata)) {
		throw new Error('Buildx metadata does not contain a valid containerimage.digest')
	}

	const digest = (metadata as Record<string, unknown>)['containerimage.digest']
	if (
		!Object.hasOwn(metadata, 'containerimage.digest') ||
		typeof digest !== 'string' ||
		!/^sha256:[0-9a-f]{64}$/.test(digest)
	) {
		throw new Error('Buildx metadata does not contain a valid containerimage.digest')
	}

	return digest
}

export const redactCommand = (command: string): string =>
	command.replace(/((?:password|token|secret|authorization)=)([^\s]+)/gi, '$1[REDACTED]')

const quoteForDisplay = (value: string): string => JSON.stringify(value)

const bakeCommand = (
	plan: PublishPlan,
	image: ImageName,
	metadataFile: string,
): { args: string[]; command: string } => {
	const args = [
		'buildx',
		'bake',
		'--file',
		'docker-bake.hcl',
		'--set',
		`${image}.args.CANTERBURY_VERSION=${plan.tags[0]}`,
		'--set',
		`${image}.args.CANTERBURY_REVISION=${plan.revision}`,
		'--set',
		`${image}.args.CANTERBURY_CREATED=${plan.created}`,
		'--set',
		`${image}.labels.org.opencontainers.image.version=${plan.tags[0]}`,
		'--set',
		`${image}.labels.org.opencontainers.image.revision=${plan.revision}`,
		'--set',
		`${image}.labels.org.opencontainers.image.created=${plan.created}`,
		'--set',
		`${image}.tags=${plan.tags.map(tag => `${imageNames[image]}:${tag}`).join(',')}`,
	]
	if (metadataFile !== '') {
		args.push('--metadata-file', metadataFile)
	}

	if (plan.mode === 'build') {
		args.push('--set', `${image}.output=type=docker`)
		args.push('--provenance=false', '--sbom=false')
	}
	if (plan.mode === 'push') {
		args.push('--push')
	}
	args.push(image)

	return { args, command: `docker ${args.map(quoteForDisplay).join(' ')}` }
}

const writeDigestOutputs = (
	digestFile: string,
	digests: Partial<Record<ImageName, string>>,
): void => {
	mkdirSync(dirname(digestFile), { recursive: true })
	writeFileSync(digestFile, `${JSON.stringify(digests, null, '\t')}\n`)

	const lines = Object.entries(digests).map(([image, digest]) => `- \`${image}\`: \`${digest}\``)
	console.log(`Immutable image digests:\n${lines.join('\n')}`)

	if (process.env.GITHUB_OUTPUT !== undefined) {
		const output = Object.entries(digests)
			.map(([image, digest]) => `${image.replace('-', '_')}_digest=${digest}`)
			.join('\n')
		writeFileSync(process.env.GITHUB_OUTPUT, `${output}\n`, { flag: 'a' })
	}

	if (process.env.GITHUB_STEP_SUMMARY !== undefined) {
		writeFileSync(
			process.env.GITHUB_STEP_SUMMARY,
			`## Published image digests\n\n${lines.join('\n')}\n`,
			{ flag: 'a' },
		)
	}
}

export const publishImages = (plan: PublishPlan, run: CommandRunner = runCommand): void => {
	if (plan.mode === 'dry-run') {
		for (const image of plan.images) {
			const { args } = bakeCommand(plan, image, '')
			const printArgs = [...args.slice(0, -1), '--print', image]
			console.log(redactCommand(`dry run: docker ${printArgs.map(quoteForDisplay).join(' ')}`))
			const graph = run('docker', printArgs)
			if (graph !== '') {
				console.log(graph)
			}
		}
		return
	}

	const digests: Partial<Record<ImageName, string>> = {}
	for (const image of plan.images) {
		const metadataFile = `${plan.digestFile}.${image}.metadata.json`
		const { args, command } = bakeCommand(plan, image, metadataFile)
		console.log(redactCommand(command))
		run('docker', args)

		if (plan.mode === 'push') {
			digests[image] = parseBuildMetadata(readFileSync(metadataFile, 'utf8'))
		}
	}

	if (plan.mode === 'push') {
		writeDigestOutputs(plan.digestFile, digests)
	}
}
