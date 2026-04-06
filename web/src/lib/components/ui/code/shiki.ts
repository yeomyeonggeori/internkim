import { createJavaScriptRegexEngine } from 'shiki/engine/javascript';
import { createHighlighterCore } from 'shiki/core';

const bundledLanguages = {
	bash: () => import('@shikijs/langs/bash'),
	c: () => import('@shikijs/langs/c'),
	css: () => import('@shikijs/langs/css'),
	diff: () => import('@shikijs/langs/diff'),
	go: () => import('@shikijs/langs/go'),
	html: () => import('@shikijs/langs/html'),
	javascript: () => import('@shikijs/langs/javascript'),
	json: () => import('@shikijs/langs/json'),
	markdown: () => import('@shikijs/langs/markdown'),
	python: () => import('@shikijs/langs/python'),
	rust: () => import('@shikijs/langs/rust'),
	sql: () => import('@shikijs/langs/sql'),
	svelte: () => import('@shikijs/langs/svelte'),
	typescript: () => import('@shikijs/langs/typescript'),
	yaml: () => import('@shikijs/langs/yaml'),
};

export type SupportedLanguage = keyof typeof bundledLanguages;

export const highlighter = createHighlighterCore({
	themes: [
		import('@shikijs/themes/github-light-default'),
		import('@shikijs/themes/github-dark-default'),
	],
	langs: Object.entries(bundledLanguages).map(([_, lang]) => lang),
	engine: createJavaScriptRegexEngine()
});
