import type { SupportedLanguage } from '$lib/components/ui/code/shiki';
import type { Component } from 'svelte';
import FileIcon from '@lucide/svelte/icons/file';
import FileTextIcon from '@lucide/svelte/icons/file-text';
import FileCodeIcon from '@lucide/svelte/icons/file-code';
import FileImageIcon from '@lucide/svelte/icons/file-image';
import FileSpreadsheetIcon from '@lucide/svelte/icons/file-spreadsheet';

export type FileVisual = {
	icon: Component;
	colorClass: string;
};

const extensionGroups: { icon: Component; colorClass: string; extensions: string[] }[] = [
	{
		icon: FileImageIcon,
		colorClass: 'text-violet-500',
		extensions: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico', 'avif']
	},
	{
		icon: FileSpreadsheetIcon,
		colorClass: 'text-emerald-600',
		extensions: ['csv', 'tsv', 'xls', 'xlsx']
	},
	{
		icon: FileCodeIcon,
		colorClass: 'text-sky-600',
		extensions: [
			'ts', 'tsx', 'js', 'jsx', 'mjs', 'cjs', 'go', 'rs', 'py', 'c', 'h', 'sh', 'bash', 'zsh',
			'svelte', 'html', 'htm', 'xml', 'css', 'json', 'jsonc', 'yaml', 'yml', 'toml', 'ini',
			'conf', 'env', 'diff', 'patch', 'sql'
		]
	},
	{
		icon: FileTextIcon,
		colorClass: 'text-amber-600',
		extensions: ['md', 'markdown', 'txt', 'text', 'log', 'pdf', 'doc', 'docx', 'rtf']
	}
];

export function fileVisual(fileName: string): FileVisual {
	const extension = fileName.split('.').pop()?.toLowerCase() ?? '';
	const group = extensionGroups.find((candidate) => candidate.extensions.includes(extension));
	if (group) return { icon: group.icon, colorClass: group.colorClass };
	return { icon: FileIcon, colorClass: 'text-muted-foreground' };
}

export function parseDelimitedText(content: string, delimiter: string): string[][] {
	return content
		.replace(/\r\n/g, '\n')
		.replace(/\n+$/, '')
		.split('\n')
		.map((line) => line.split(delimiter));
}

export function formatFileSize(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	const units = ['KB', 'MB', 'GB', 'TB'];
	let size = bytes / 1024;
	let unitIndex = 0;
	while (size >= 1024 && unitIndex < units.length - 1) {
		size /= 1024;
		unitIndex += 1;
	}
	const rounded = size >= 10 ? Math.round(size) : Math.round(size * 10) / 10;
	return `${rounded} ${units[unitIndex]}`;
}

export function formatModified(isoTimestamp: string, locale: 'ko' | 'en'): string {
	const modified = new Date(isoTimestamp);
	if (Number.isNaN(modified.getTime())) return '';
	const dayInMilliseconds = 24 * 60 * 60 * 1000;
	const startOfToday = new Date();
	startOfToday.setHours(0, 0, 0, 0);
	const dayDifference = Math.floor((startOfToday.getTime() - modified.getTime()) / dayInMilliseconds) + 1;

	if (dayDifference <= 0) return locale === 'ko' ? '오늘' : 'Today';
	if (dayDifference === 1) return locale === 'ko' ? '어제' : 'Yesterday';
	if (dayDifference < 7) return locale === 'ko' ? `${dayDifference}일 전` : `${dayDifference} days ago`;
	return modified.toLocaleDateString(locale === 'ko' ? 'ko-KR' : 'en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}


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
