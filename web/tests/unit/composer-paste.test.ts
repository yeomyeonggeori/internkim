import { describe, expect, test } from 'bun:test';
import { getSchema } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Fragment, Slice } from '@tiptap/pm/model';
import { markdownOfComposer } from '../../src/lib/components/channel/composer-markdown';
import { pastedParagraphsAsCopiedLines, pastedTextSlice } from '../../src/lib/components/channel/composer-paste';

const schema = getSchema([StarterKit.configure({ heading: false, horizontalRule: false, underline: false })]);
const insideParagraph = schema.nodes.doc.create(null, schema.nodes.paragraph.create()).resolve(1);
const insideCode = schema.nodes.doc.create(null, schema.nodes.codeBlock.create()).resolve(1);
const original = 'Q. 첫 질문\n답 첫 줄\n답 둘째 줄\n\nQ. 둘째 질문\n답';

function sentText(slice: Slice | undefined): string {
	return markdownOfComposer(schema.nodes.doc.create(null, slice?.content).toJSON());
}

function paragraphsSlice(lines: string[]): Slice {
	const blocks = lines.map((line) => schema.nodes.paragraph.create(null, line === '' ? [] : [schema.text(line)]));
	return new Slice(Fragment.from(blocks), 1, 1);
}

describe('pasted plain text keeps its line breaks', () => {
	test('is sent exactly as it was copied', () => {
		expect(sentText(pastedTextSlice(original, insideParagraph, schema))).toBe(original);
	});

	test('Windows line endings become ordinary line breaks', () => {
		expect(sentText(pastedTextSlice('가\r\n나', insideParagraph, schema))).toBe('가\n나');
	});

	test('inside a code block keeps its newlines as text', () => {
		expect(pastedTextSlice('a\nb', insideCode, schema).content.firstChild?.text).toBe('a\nb');
	});
});

describe('pasted paragraphs follow the lines the source copied', () => {
	test('a document that stores one paragraph per line, blank ones included, is sent as its lines', () => {
		const pasted = paragraphsSlice(['Q. 첫 질문', '답 첫 줄', '답 둘째 줄', '', 'Q. 둘째 질문', '답']);
		expect(sentText(pastedParagraphsAsCopiedLines(pasted, original, insideParagraph, schema))).toBe(original);
	});

	test('a web page or message whose paragraphs copy with a blank line between keeps that blank line', () => {
		const pasted = paragraphsSlice(['가', '나']);
		expect(sentText(pastedParagraphsAsCopiedLines(pasted, '가\n\n나', insideParagraph, schema))).toBe('가\n\n나');
	});

	test('formatting inside the paragraphs survives', () => {
		const bold = schema.nodes.paragraph.create(null, schema.text('굵게', [schema.marks.bold.create()]));
		const pasted = new Slice(Fragment.from([bold, schema.nodes.paragraph.create(null, schema.text('보통'))]), 1, 1);
		expect(sentText(pastedParagraphsAsCopiedLines(pasted, '굵게\n보통', insideParagraph, schema))).toBe('**굵게**\n보통');
	});

	test('paragraphs that do not line up with the copied text fall back to the copied text', () => {
		const pasted = paragraphsSlice(['가', '나']);
		expect(sentText(pastedParagraphsAsCopiedLines(pasted, '가\n다', insideParagraph, schema))).toBe('가\n다');
	});

	test('a list pasted with a paragraph is left to the editor', () => {
		const list = schema.nodes.bulletList.create(null, [
			schema.nodes.listItem.create(null, schema.nodes.paragraph.create(null, schema.text('항목')))
		]);
		const pasted = new Slice(Fragment.from([schema.nodes.paragraph.create(null, schema.text('제목')), list]), 0, 0);
		expect(pastedParagraphsAsCopiedLines(pasted, '제목\n항목', insideParagraph, schema)).toBeUndefined();
	});
});
