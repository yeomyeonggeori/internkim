import { describe, expect, test } from 'bun:test';
import StarterKit from '@tiptap/starter-kit';
import { MarkdownManager } from '@tiptap/markdown';
import type { JSONContent } from '@tiptap/core';
import { composerDocumentOf, markdownOfComposer } from '../../src/lib/components/channel/composer-markdown';

const parser = new MarkdownManager({
	extensions: [StarterKit.configure({ heading: false, horizontalRule: false, underline: false })]
});
const parse = (markdown: string): JSONContent => parser.parse(markdown);

function paragraph(...content: JSONContent[]): JSONContent {
	return { type: 'doc', content: [{ type: 'paragraph', content }] };
}

describe('markdownOfComposer', () => {
	test('writes typed characters as they are, without escaping them', () => {
		expect(markdownOfComposer(paragraph({ type: 'text', text: 'report_final.pdf 2*3 a < b' }))).toBe(
			'report_final.pdf 2*3 a < b'
		);
	});

	test('keeps spaces outside the markers so the formatting still renders', () => {
		const written = markdownOfComposer(
			paragraph({ type: 'text', text: '굵게 ', marks: [{ type: 'bold' }] }, { type: 'text', text: '보통' })
		);
		expect(written).toBe('**굵게** 보통');
	});

	test('opens and closes overlapping marks once each', () => {
		const written = markdownOfComposer(
			paragraph(
				{ type: 'text', text: 'a', marks: [{ type: 'bold' }] },
				{ type: 'text', text: 'b', marks: [{ type: 'bold' }, { type: 'italic' }] }
			)
		);
		expect(written).toBe('**a*b***');
	});

	test('writes a link whose text is its address as the bare address', () => {
		const address = 'https://example.com/a_b';
		const written = markdownOfComposer(
			paragraph({ type: 'text', text: address, marks: [{ type: 'link', attrs: { href: address } }] })
		);
		expect(written).toBe(address);
	});

	test('writes a line break as a newline', () => {
		const written = markdownOfComposer(
			paragraph({ type: 'text', text: '첫 줄' }, { type: 'hardBreak' }, { type: 'text', text: '둘째 줄' })
		);
		expect(written).toBe('첫 줄\n둘째 줄');
	});
});

describe('composerDocumentOf', () => {
	test.each([
		'**굵게** 보통',
		'*기울임* ~~취소~~ `코드`',
		'[링크](https://example.com)',
		'- 하나\n- **둘**',
		'1. 하나\n2. 둘',
		'> 인용\n> 둘째',
		'```\nconst x = 1\n```',
		'첫 줄\n둘째 줄'
	])('shows %p formatted and writes it back unchanged', (markdown) => {
		const shown = composerDocumentOf(markdown, parse);
		expect(shown).toEqual(parse(markdown));
		expect(markdownOfComposer(shown)).toBe(markdown);
	});

	test.each([
		'# 제목',
		'| a | b |\n| - | - |\n| 1 | 2 |',
		'![사진](https://example.com/a.png)',
		'위\n\n---\n\n아래',
		'- [ ] 할 일',
		'snake\\_case'
	])('keeps %p as plain text when formatting it would lose something', (markdown) => {
		const shown = composerDocumentOf(markdown, parse);
		expect(shown).toEqual(paragraph({ type: 'text', text: markdown }));
		expect(markdownOfComposer(shown)).toBe(markdown);
	});
});
