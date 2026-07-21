const js = require('@eslint/js')
const prettier = require('eslint-config-prettier')
const globals = require('globals')
const tseslint = require('typescript-eslint')

module.exports = tseslint.config(
	{
		ignores: ['**/node_modules/**'],
	},
	js.configs.recommended,
	...tseslint.configs.recommended,
	{
		files: ['**/*.js', '**/*.mts'],
		languageOptions: {
			ecmaVersion: 'latest',
			globals: {
				...globals.nodeBuiltin,
				...globals.node,
			},
			sourceType: 'module',
		},
		rules: {
			'arrow-body-style': ['error', 'as-needed'],
			'no-restricted-syntax': [
				'error',
				{
					selector: 'FunctionDeclaration',
					message: 'Use a const arrow function instead of a named function declaration.',
				},
			],
		},
	},
	prettier,
)
