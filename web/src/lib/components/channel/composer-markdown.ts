import type { JSONContent } from '@tiptap/core';

type InlineMark = 'bold' | 'italic' | 'strike' | 'link';

const inlineMarkOrder: InlineMark[] = ['link', 'bold', 'italic', 'strike'];

const markDelimiters: Record<Exclude<InlineMark, 'link'>, string> = {
	bold: '**',
	italic: '*',
	strike: '~~'
};

type InlineRun = { text: string; marks: InlineMark[]; href: string; isCode: boolean };

class UnwritableComposerNode extends Error {
	constructor(nodeType: string | undefined) {
		super(`composer-markdown: no markdown for "${nodeType}"; add it here or remove it from the editor`);
	}
}

export function markdownOfComposer(document: JSONContent): string {
	return blocksOf(document.content ?? [], '\n\n');
}

export function composerDocumentOf(text: string, parseMarkdown: (markdown: string) => JSONContent): JSONContent {
	if (text === '') return { type: 'doc', content: [{ type: 'paragraph' }] };
	const parsed = parseMarkdown(text);
	if (writesBackUnchanged(parsed, text)) return parsed;
	return { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] };
}

function writesBackUnchanged(parsed: JSONContent, text: string): boolean {
	try {
		return markdownOfComposer(parsed) === text;
	} catch (error) {
		if (error instanceof UnwritableComposerNode) return false;
		throw error;
	}
}

function blocksOf(blocks: JSONContent[], separator: string): string {
	return blocks.map(blockOf).join(separator);
}

function blockOf(block: JSONContent): string {
	const children = block.content ?? [];
	if (block.type === 'paragraph') return inlineOf(children);
	if (block.type === 'bulletList') return children.map((item) => listItemOf(item, '- ')).join('\n');
	if (block.type === 'orderedList') {
		const start = typeof block.attrs?.start === 'number' ? block.attrs.start : 1;
		return children.map((item, index) => listItemOf(item, `${start + index}. `)).join('\n');
	}
	if (block.type === 'blockquote') return prefixedLines(blocksOf(children, '\n\n'), '> ', '> ');
	if (block.type === 'codeBlock') {
		const language = typeof block.attrs?.language === 'string' ? block.attrs.language : '';
		return '```' + language + '\n' + children.map((child) => child.text ?? '').join('') + '\n```';
	}
	throw new UnwritableComposerNode(block.type);
}

function listItemOf(item: JSONContent, marker: string): string {
	return prefixedLines(blocksOf(item.content ?? [], '\n'), marker, ' '.repeat(marker.length));
}

function prefixedLines(text: string, first: string, rest: string): string {
	return text
		.split('\n')
		.map((line, index) => {
			const prefix = index === 0 ? first : rest;
			return line === '' ? prefix.trimEnd() : prefix + line;
		})
		.join('\n');
}

function inlineOf(nodes: JSONContent[]): string {
	let written = '';
	let open: InlineRun | undefined;
	for (const run of nodes.flatMap(runsOf)) {
		written += transition(open, run) + (run.isCode ? '`' + run.text + '`' : run.text);
		open = run;
	}
	return written + transition(open, undefined);
}

function runsOf(node: JSONContent): InlineRun[] {
	if (node.type === 'hardBreak') return [{ text: '\n', marks: [], href: '', isCode: false }];
	if (node.type !== 'text') {
		throw new UnwritableComposerNode(node.type);
	}
	const text = node.text ?? '';
	const types = (node.marks ?? []).map((mark) => mark.type);
	const linkMark = (node.marks ?? []).find((mark) => mark.type === 'link');
	const href = typeof linkMark?.attrs?.href === 'string' ? linkMark.attrs.href : '';
	const isCode = types.includes('code');
	const marks = inlineMarkOrder.filter((mark) => types.includes(mark) && !(mark === 'link' && href === text));
	if (isCode || marks.length === 0) return [{ text, marks, href, isCode }];
	const leading = text.match(/^\s*/)?.[0] ?? '';
	const trailing = text.slice(leading.length).match(/\s*$/)?.[0] ?? '';
	const core = text.slice(leading.length, text.length - trailing.length);
	const plain = (spaces: string): InlineRun[] => (spaces ? [{ text: spaces, marks: [], href: '', isCode: false }] : []);
	if (core === '') return plain(text);
	return [...plain(leading), { text: core, marks, href, isCode }, ...plain(trailing)];
}

function transition(previous: InlineRun | undefined, next: InlineRun | undefined): string {
	const before = previous?.marks ?? [];
	const after = next?.marks ?? [];
	let shared = 0;
	while (
		shared < before.length &&
		shared < after.length &&
		before[shared] === after[shared] &&
		(before[shared] !== 'link' || previous?.href === next?.href)
	) {
		shared++;
	}
	const closing = before.slice(shared).reverse().map((mark) => closingOf(mark, previous?.href ?? '')).join('');
	const opening = after.slice(shared).map(openingOf).join('');
	return closing + opening;
}

function openingOf(mark: InlineMark): string {
	return mark === 'link' ? '[' : markDelimiters[mark];
}

function closingOf(mark: InlineMark, href: string): string {
	return mark === 'link' ? `](${href})` : markDelimiters[mark];
}
