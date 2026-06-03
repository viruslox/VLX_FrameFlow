import eslintPluginSvelte from "eslint-plugin-svelte";
export default [
  ...eslintPluginSvelte.configs["flat/recommended"],
  {
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
    },
  },
];
