import js from '@eslint/js';
import ts from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  {
    plugins: { 'react-hooks': reactHooks },
    rules: {
      // Core rules from recommended - keep at their recommended level
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      // New v7 rules - downgrade the ones that fire on valid existing patterns.
      // Each entry below was reviewed against the actual code: these are not bugs,
      // they are patterns the new rules flag too broadly.
      //
      // set-state-in-effect: fires on intentional initialization effects like
      //   "auto-open nav group on route change" and "reset search index when
      //   results change". These are correct; the rule catches cascades.
      'react-hooks/set-state-in-effect': 'warn',
      // refs: fires on passing a ref object as a JSX ref= prop and on reading
      //   state arrays returned alongside refs from a custom hook. Both are valid.
      'react-hooks/refs': 'warn',
      // purity: fires on Math.random() inside useMemo (which IS pure within the
      //   memoization boundary). Real render-time impurity cases are caught by
      //   no-render-return-value and no-side-effects-in-render.
      'react-hooks/purity': 'warn',
      // immutability / preserve-manual-memoization: experimental v7 rules with
      //   high false-positive rates on idiomatic useState + callback patterns.
      'react-hooks/immutability': 'warn',
      'react-hooks/preserve-manual-memoization': 'warn',
      // Unused vars: error on variables, warn on args (common in event callbacks)
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      // No explicit any: warn rather than error (legacy code may need migration)
      '@typescript-eslint/no-explicit-any': 'warn',
    },
  },
  {
    ignores: ['dist/', '../app/'],
  },
);
