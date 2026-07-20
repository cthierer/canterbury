const js = require('./sync/node_modules/@eslint/js')
const prettier = require('./sync/node_modules/eslint-config-prettier')
const globals = require('./sync/node_modules/globals')

module.exports = [
	{
		ignores: ['**/node_modules/**'],
	},
	js.configs.recommended,
	{
		files: ['sync/**/*.js', 'scripts/**/*.mjs'],
		languageOptions: {
			ecmaVersion: 'latest',
			globals: {
				...globals.nodeBuiltin,
				...globals.node,
			},
			sourceType: 'module',
		},
		rules: {
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
]
