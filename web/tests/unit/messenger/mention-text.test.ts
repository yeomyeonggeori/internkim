import { describe, expect, test } from 'bun:test';
import { mentionPieces } from '$lib/messenger/mention-text';

describe('mentionPieces', () => {
	test('marks the name the message says it called on', () => {
		expect(mentionPieces('안녕 @이샘플 봤어?', [{ label: '이샘플', externalID: 'external-1' }])).toEqual([
			{ text: '안녕 ', isMention: false },
			{ text: '@이샘플', isMention: true, externalID: 'external-1' },
			{ text: ' 봤어?', isMention: false }
		]);
	});

	test('leaves a name the message did not call on as words', () => {
		expect(mentionPieces('@박예시 안녕', [{ label: '이샘플' }])).toEqual([{ text: '@박예시 안녕', isMention: false }]);
	});

	test('a message that called on nobody is one plain piece', () => {
		expect(mentionPieces('안녕', [])).toEqual([{ text: '안녕', isMention: false }]);
	});

	test('prefers the longer of two names that start alike', () => {
		expect(mentionPieces('@이샘플이', [{ label: '이샘플' }, { label: '이샘플이' }])).toEqual([
			{ text: '@이샘플이', isMention: true }
		]);
	});

	test('marks every time the name appears', () => {
		expect(mentionPieces('@all @all', [{ label: 'all' }])).toEqual([
			{ text: '@all', isMention: true },
			{ text: ' ', isMention: false },
			{ text: '@all', isMention: true }
		]);
	});
});
