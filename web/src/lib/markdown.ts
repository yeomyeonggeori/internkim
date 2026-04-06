import { marked, type Token, type Tokens } from 'marked';
import DOMPurify from 'dompurify';

export type HtmlSegment = { type: 'html'; content: string };
export type CodeSegment = { type: 'code'; lang: string; code: string };
export type Segment = HtmlSegment | CodeSegment;

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

function createRenderer(): marked.Renderer {
	const renderer = new marked.Renderer();
	renderer.image = ({ href, text: alt }) => {
		if (isFileLink(href)) {
			return `<span class="file-preview file-preview-image expandable-image" data-src="${href}"><img src="${href}" alt="${alt || ''}" /></span>`;
		}
		return `<img src="${href}" alt="${alt || ''}" class="expandable-image" data-src="${href}" />`;
	};
	renderer.link = ({ href, title, text: linkText }) => {
		if (isFileLink(href)) {
			const extension = getFileExtension(href);
			const fileName = getFileName(href);
			if (imageExtensions.has(extension)) {
				return `<span class="file-preview file-preview-image expandable-image" data-src="${href}"><img src="${href}" alt="${fileName}" /></span>`;
			}
			return `<a href="${href}" download class="file-preview file-preview-doc" target="_blank"><span class="file-icon">📄</span><span class="file-info"><span class="file-name">${fileName}</span><span class="file-ext">${extension.toUpperCase()}</span></span><span class="file-action">↓</span></a>`;
		}
		return `<a href="${href}" title="${title || ''}" target="_blank" rel="noopener">${linkText}</a>`;
	};
	return renderer;
}

function flushHtmlTokens(tokens: Token[]): HtmlSegment | null {
	if (tokens.length === 0) return null;
	const tokenList = tokens as Token[] & { links: Record<string, { href: string; title: string }> };
	tokenList.links = {};
	const raw = marked.parser(tokenList, { renderer: createRenderer() });
	const sanitized = DOMPurify.sanitize(raw, { ADD_ATTR: ['data-src'] });
	return sanitized.trim() ? { type: 'html', content: sanitized } : null;
}

export function parseMarkdownSegments(text: string): Segment[] {
	const tokens = marked.lexer(text);
	const segments: Segment[] = [];
	let buffer: Token[] = [];
	for (const token of tokens) {
		if (token.type === 'code') {
			const htmlSegment = flushHtmlTokens(buffer);
			if (htmlSegment) segments.push(htmlSegment);
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
	const htmlSegment = flushHtmlTokens(buffer);
	if (htmlSegment) segments.push(htmlSegment);
	return segments;
}
