import { marked, type Token, type Tokens } from 'marked';
import DOMPurify from 'dompurify';

export type HtmlSegment = { type: 'html'; content: string };
export type CodeSegment = { type: 'code'; lang: string; code: string };
export type FileSegment = { type: 'file'; href: string; name: string; ext: string; isImage: boolean };
export type Segment = HtmlSegment | CodeSegment | FileSegment;

const imageExtensions = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg']);

function isFileLink(href: string): boolean {
	return href.startsWith('/pico/files/');
}

function getFileExtension(path: string): string {
	return path.split('.').pop()?.toLowerCase() || '';
}

function getFileName(path: string): string {
	return path.split('/').pop() || path;
}

const FILE_PLACEHOLDER = '__PICO_FILE__';

function createRenderer(fileSegments: FileSegment[]): marked.Renderer {
	const renderer = new marked.Renderer();
	renderer.image = ({ href, text: alt }) => {
		if (isFileLink(href)) {
			const ext = getFileExtension(href);
			const name = getFileName(href);
			fileSegments.push({ type: 'file', href, name: alt || name, ext, isImage: true });
			return `<span data-pico-file="${fileSegments.length - 1}">${FILE_PLACEHOLDER}</span>`;
		}
		return `<img src="${href}" alt="${alt || ''}" class="expandable-image" data-src="${href}" />`;
	};
	renderer.link = ({ href, title, text: linkText }) => {
		if (isFileLink(href)) {
			const ext = getFileExtension(href);
			const name = getFileName(href);
			const isImage = imageExtensions.has(ext);
			fileSegments.push({ type: 'file', href, name: linkText || name, ext, isImage });
			return `<span data-pico-file="${fileSegments.length - 1}">${FILE_PLACEHOLDER}</span>`;
		}
		return `<a href="${href}" title="${title || ''}" target="_blank" rel="noopener">${linkText}</a>`;
	};
	return renderer;
}

function flushHtmlTokens(tokens: Token[], fileSegments: FileSegment[]): Segment[] {
	if (tokens.length === 0) return [];
	const tokenList = tokens as Token[] & { links: Record<string, { href: string; title: string }> };
	tokenList.links = {};
	const raw = marked.parser(tokenList, { renderer: createRenderer(fileSegments) });
	const sanitized = DOMPurify.sanitize(raw, { ADD_ATTR: ['data-src', 'data-pico-file'] });
	if (!sanitized.trim()) return [];

	// Split on file placeholders and emit FileSegments inline
	const result: Segment[] = [];
	const parts = sanitized.split(/<span data-pico-file="(\d+)">__PICO_FILE__<\/span>/);
	for (let i = 0; i < parts.length; i++) {
		if (i % 2 === 0) {
			if (parts[i].trim()) result.push({ type: 'html', content: parts[i] });
		} else {
			const seg = fileSegments[parseInt(parts[i])];
			if (seg) result.push(seg);
		}
	}
	return result;
}

export function parseMarkdownSegments(text: string): Segment[] {
	const tokens = marked.lexer(text);
	const segments: Segment[] = [];
	const fileSegments: FileSegment[] = [];
	let buffer: Token[] = [];
	for (const token of tokens) {
		if (token.type === 'code') {
			segments.push(...flushHtmlTokens(buffer, fileSegments));
			buffer = [];
			segments.push({
				type: 'code',
				lang: (token as Tokens.Code).lang || '',
				code: (token as Tokens.Code).text
			});
		} else {
			buffer.push(token);
		}
	}
	segments.push(...flushHtmlTokens(buffer, fileSegments));
	return segments;
}
