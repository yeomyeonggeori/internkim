import { describe, expect, test } from 'bun:test';
import {
	applyComposerFormat,
	insertIntoDraft,
	type ComposerDraft
} from '../../src/lib/components/channel/composer-formatting';

function draftOf(text: string, selectionStart: number, selectionEnd = selectionStart): ComposerDraft {
	return { text, selectionStart, selectionEnd };
}

describe('applyComposerFormat', () => {
	test('wraps the selected words and keeps them selected', () => {
		expect(applyComposerFormat(draftOf('say hello now', 4, 9), 'bold')).toEqual(draftOf('say **hello** now', 6, 11));
		expect(applyComposerFormat(draftOf('hello', 0, 5), 'italic')).toEqual(draftOf('*hello*', 1, 6));
		expect(applyComposerFormat(draftOf('hello', 0, 5), 'strikethrough')).toEqual(draftOf('~~hello~~', 2, 7));
	});

	test('marks part of a word with a marker markdown accepts inside a word', () => {
		expect(applyComposerFormat(draftOf('안녕하세요', 0, 2), 'italic')).toEqual(draftOf('*안녕*하세요', 1, 3));
	});

	test('puts the cursor between the markers when nothing is selected', () => {
		expect(applyComposerFormat(draftOf('ab', 1), 'bold')).toEqual(draftOf('a****b', 3));
	});

	test('turns the selection into a link label and leaves the cursor where the address goes', () => {
		expect(applyComposerFormat(draftOf('docs', 0, 4), 'link')).toEqual(draftOf('[docs]()', 7));
		expect(applyComposerFormat(draftOf('', 0), 'link')).toEqual(draftOf('[]()', 1));
	});

	test('uses inline code for one line and a fenced block for several', () => {
		expect(applyComposerFormat(draftOf('run make', 4, 8), 'code')).toEqual(draftOf('run `make`', 5, 9));
		expect(applyComposerFormat(draftOf('a\nb', 0, 3), 'code')).toEqual(draftOf('```\na\nb\n```', 4, 7));
	});

	test('prefixes every line the selection touches', () => {
		expect(applyComposerFormat(draftOf('one\ntwo\nthree', 1, 5), 'orderedList')).toEqual(
			draftOf('1. one\n2. two\nthree', 0, 13)
		);
		expect(applyComposerFormat(draftOf('one\ntwo', 5), 'bulletList')).toEqual(draftOf('one\n- two', 7));
		expect(applyComposerFormat(draftOf('said', 0, 4), 'quote')).toEqual(draftOf('> said', 0, 6));
	});
});

describe('insertIntoDraft', () => {
	test('replaces the selection and puts the cursor after what was inserted', () => {
		expect(insertIntoDraft(draftOf('hi there', 3, 8), '👋')).toEqual(draftOf('hi 👋', 5));
	});
});
