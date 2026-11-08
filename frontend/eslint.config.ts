import js from '@eslint/js';
import boundaries from 'eslint-plugin-boundaries';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'coverage'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  jsxA11y.flatConfigs.recommended,
  reactHooks.configs.flat.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: { globals: globals.browser },
  },
  // Dependency rules of docs/Architecture/Frontend.md §3.
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { boundaries },
    settings: {
      'import/resolver': { typescript: { alwaysTryTypes: true } },
      'boundaries/elements': [
        { type: 'app', pattern: 'src/app/**' },
        { type: 'admin', pattern: 'src/features/admin' },
        { type: 'feature', pattern: 'src/features/*', capture: ['name'] },
        { type: 'shared', pattern: 'src/shared/**' },
      ],
      'boundaries/include': ['src/**/*'],
    },
    rules: {
      'boundaries/dependencies': [
        'error',
        {
          default: 'disallow',
          policies: [
            { from: { element: { type: 'shared' } }, allow: { to: { element: { type: 'shared' } } } },
            {
              from: { element: { type: 'feature' } },
              allow: [
                { to: { element: { type: 'shared' } } },
                { to: { element: { type: 'feature', captured: { name: '{{ from.element.captured.name }}' } } } },
              ],
            },
            {
              from: { element: { type: 'admin' } },
              allow: { to: { element: { types: ['shared', 'feature', 'admin'] } } },
            },
            {
              from: { element: { type: 'app' } },
              allow: { to: { element: { types: ['shared', 'feature', 'admin', 'app'] } } },
            },
          ],
        },
      ],
    },
  },
  {
    files: ['**/*.config.ts', 'scripts/**'],
    languageOptions: { globals: globals.node },
  },
);
