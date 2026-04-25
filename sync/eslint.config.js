import js from '@eslint/js'
import prettier from 'eslint-config-prettier'
import globals from 'globals'

export default [
	{
		ignores: ['node_modules/**'],
	},
	js.configs.recommended,
	{
		files: ['**/*.js'],
		languageOptions: {
			ecmaVersion: 'latest',
			globals: {
				...globals.nodeBuiltin,
				...globals.node,
			},
			sourceType: 'module',
		},
	},
	prettier,
]
