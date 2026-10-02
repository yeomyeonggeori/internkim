import { describe, expect, test } from 'bun:test';
import {
	mentionKeyAction,
	mentionFragmentAt,
	mentionsToSend,
	mentionWritten,
	type ChosenMention
} from '$lib/messenger/mention-draft';

describe('mentionFragmentAt', () => {
	test('opens on an @ that starts the line', () => {
		expect(mentionFragmentAt('@sam', 4)).toEqual({ start: 0, query: 'sam' });
	});

	test('opens on an @ that starts a word', () => {
		expect(mentionFragmentAt('안녕 @sam', 7)).toEqual({ start: 3, query: 'sam' });
	});

	test('stays closed inside an address', () => {
		expect(mentionFragmentAt('sample@example.com', 18)).toBeUndefined();
	});

	test('closes once the query runs past a space', () => {
		expect(mentionFragmentAt('@sam 안녕', 8)).toBeUndefined();
	});

	test('reads only what is left of the cursor', () => {
		expect(mentionFragmentAt('@sample', 3)).toEqual({ start: 0, query: 'sa' });
	});

	test('a bare @ opens the list with nothing typed', () => {
		expect(mentionFragmentAt('@', 1)).toEqual({ start: 0, query: '' });
	});
});

describe('mentionWritten', () => {
	test('replaces the typed fragment up to the cursor with the name', () => {
		const fragment = { start: 3, query: 'sa' };
		expect(mentionWritten(6, fragment, '이샘플')).toEqual({ from: 3, to: 6, inserted: '@이샘플 ' });
	});
});

describe('mentionsToSend', () => {
	const chosen: ChosenMention[] = [
		{ key: 'external-1', label: '이샘플', externalID: 'external-1', isEveryone: false },
		{ key: 'everyone', label: 'all', isEveryone: true }
	];

	test('sends what the draft still names', () => {
		expect(mentionsToSend('@이샘플 안녕', chosen)).toEqual({
			externalIDs: ['external-1'],
			isEveryone: false
		});
	});

	test('drops what the author deleted again', () => {
		expect(mentionsToSend('안녕', chosen)).toEqual({ externalIDs: [], isEveryone: false });
	});

	test('carries a call on everyone', () => {
		expect(mentionsToSend('@all 안녕', chosen)).toEqual({ externalIDs: [], isEveryone: true });
	});

	test('names a person once however often they were picked', () => {
		const twice = [chosen[0], { ...chosen[0], key: 'external-1-again' }];
		expect(mentionsToSend('@이샘플 @이샘플', twice).externalIDs).toEqual(['external-1']);
	});
});

describe('mentionKeyAction', () => {
	test('the arrows and Enter belong to the open list', () => {
		expect(mentionKeyAction('ArrowDown', false)).toBe('down');
		expect(mentionKeyAction('ArrowUp', false)).toBe('up');
		expect(mentionKeyAction('Enter', false)).toBe('take');
		expect(mentionKeyAction('Tab', false)).toBe('take');
		expect(mentionKeyAction('Escape', false)).toBe('close');
	});

	test('an ordinary letter belongs to the message', () => {
		expect(mentionKeyAction('ㄱ', false)).toBeUndefined();
	});

	test('a key still being composed into a letter belongs to the input method', () => {
		expect(mentionKeyAction('Enter', true)).toBeUndefined();
	});
});
