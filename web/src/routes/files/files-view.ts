import type { SupportedLanguage } from '$lib/components/ui/code/shiki';

const languageByExtension: Record<string, SupportedLanguage> = {
	html: 'html',
	htm: 'html',
	xml: 'html',
	svg: 'html',
	css: 'css',
	js: 'javascript',
	mjs: 'javascript',
	cjs: 'javascript',
	jsx: 'javascript',
	ts: 'typescript',
	mts: 'typescript',
	cts: 'typescript',
	tsx: 'typescript',
	json: 'json',
	jsonc: 'json',
	py: 'python',
	go: 'go',
	rs: 'rust',
	sql: 'sql',
	svelte: 'svelte',
	yaml: 'yaml',
	yml: 'yaml',
	sh: 'bash',
	bash: 'bash',
	zsh: 'bash',
	c: 'c',
	h: 'c',
	diff: 'diff',
	patch: 'diff',
	md: 'markdown',
	markdown: 'markdown',
	txt: 'markdown',
	text: 'markdown',
	log: 'markdown',
	csv: 'markdown',
	tsv: 'markdown',
	env: 'bash',
	ini: 'bash',
	conf: 'bash',
	toml: 'bash'
};

export function codeLanguageForFile(fileName: string): SupportedLanguage | null {
	const extension = fileName.split('.').pop()?.toLowerCase() ?? '';
	return languageByExtension[extension] ?? null;
}
