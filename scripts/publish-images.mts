import { publishImages, resolvePublishPlan } from './shared/publish-images.mts'

if (process.argv.length !== 2) {
	throw new Error(
		'publish-images accepts no command-line arguments; use REF, IMAGE, and MODE environment variables',
	)
}

const plan = resolvePublishPlan({
	digestFile: process.env.IMAGE_DIGESTS_FILE,
	image: process.env.IMAGE,
	mode: process.env.MODE,
	ref: process.env.REF,
})

console.log(
	`Preparing ${plan.mode} for ${plan.images.join(', ')} at ${plan.revision} with tags ${plan.tags.join(', ')}`,
)
publishImages(plan)
