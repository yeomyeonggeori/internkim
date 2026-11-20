import type { SupportedLanguage } from '$lib/components/ui/code/shiki';
import type { Component } from 'svelte';
import FileIcon from '@lucide/svelte/icons/file';
import FileTextIcon from '@lucide/svelte/icons/file-text';
import FileCodeIcon from '@lucide/svelte/icons/file-code';
import FileArchiveIcon from '@lucide/svelte/icons/file-archive';
import FileImageIcon from '@lucide/svelte/icons/file-image';
import FileMusicIcon from '@lucide/svelte/icons/file-music';
import FilePlayIcon from '@lucide/svelte/icons/file-play';
import FileSpreadsheetIcon from '@lucide/svelte/icons/file-spreadsheet';
import PresentationIcon from '@lucide/svelte/icons/presentation';

export type FileVisual = {
	icon: Component;
	colorClass: string;
	tileClass: string;
};

const extensionGroups: (FileVisual & { extensions: string[] })[] = [
	{
		icon: FileImageIcon,
		colorClass: 'text-violet-500',
		tileClass: 'bg-violet-500',
		extensions: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico', 'avif', 'heic', 'heif', 'tif', 'tiff']
	},
	{
		icon: FileSpreadsheetIcon,
		colorClass: 'text-emerald-600',
		tileClass: 'bg-emerald-600',
		extensions: ['csv', 'tsv', 'xls', 'xlsx', 'xlsm', 'ods', 'numbers']
	},
	{
		icon: PresentationIcon,
		colorClass: 'text-orange-600',
		tileClass: 'bg-orange-600',
		extensions: ['ppt', 'pptx', 'key', 'odp']
	},
	{
		icon: FileCodeIcon,
		colorClass: 'text-sky-600',
		tileClass: 'bg-sky-600',
		extensions: [
			'ts', 'tsx', 'js', 'jsx', 'mjs', 'cjs', 'go', 'rs', 'py', 'c', 'h', 'sh', 'bash', 'zsh',
			'svelte', 'html', 'htm', 'xml', 'css', 'json', 'jsonc', 'yaml', 'yml', 'toml', 'ini',
			'conf', 'env', 'diff', 'patch', 'sql'
		]
	},
	{
		icon: FileIcon,
		colorClass: 'text-red-600',
		tileClass: 'bg-red-600',
		extensions: ['pdf']
	},
	{
		icon: FileTextIcon,
		colorClass: 'text-amber-600',
		tileClass: 'bg-amber-600',
		extensions: ['md', 'markdown', 'txt', 'text', 'log', 'doc', 'docx', 'rtf', 'odt', 'pages', 'hwp', 'hwpx']
	},
	{
		icon: FilePlayIcon,
		colorClass: 'text-rose-600',
		tileClass: 'bg-rose-600',
		extensions: ['mp4', 'mov', 'm4v', 'webm', 'mkv', 'avi']
	},
	{
		icon: FileMusicIcon,
		colorClass: 'text-fuchsia-600',
		tileClass: 'bg-fuchsia-600',
		extensions: ['mp3', 'wav', 'm4a', 'aac', 'ogg', 'flac']
	},
	{
		icon: FileArchiveIcon,
		colorClass: 'text-stone-500',
		tileClass: 'bg-stone-500',
		extensions: ['zip', 'tar', 'gz', 'tgz', '7z', 'rar']
	}
];

export function fileVisual(fileName: string): FileVisual {
	const extension = fileName.split('.').pop()?.toLowerCase() ?? '';
	const group = extensionGroups.find((candidate) => candidate.extensions.includes(extension));
	if (group) return { icon: group.icon, colorClass: group.colorClass, tileClass: group.tileClass };
	return { icon: FileIcon, colorClass: 'text-muted-foreground', tileClass: 'bg-muted-foreground' };
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
	xml: 'html',
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
	env: 'bash',
	ini: 'bash',
	conf: 'bash',
	toml: 'bash'
};

export function codeLanguageForFile(fileName: string): SupportedLanguage | null {
	const extension = fileName.split('.').pop()?.toLowerCase() ?? '';
	return languageByExtension[extension] ?? null;
}
