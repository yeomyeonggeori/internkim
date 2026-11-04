import { describe, expect, test } from 'bun:test';
import { reactionPeopleLabel, reactionsAfter, reactionsWithValues } from '$lib/components/channel/channel-reactions';
import { channelText } from '$lib/i18n/channel-text';

const koreanTemplate = channelText.ko.reactedBy;
const englishTemplate = channelText.en.reactedBy;

describe('reactionPeopleLabel', () => {
	test('formats a single name', () => {
		expect(reactionPeopleLabel(['이샘플'], koreanTemplate)).toBe('이샘플님이 눌렀어요');
	});

	test('formats three names', () => {
		expect(reactionPeopleLabel(['이샘플', '박예시', '최견본'], koreanTemplate)).toBe(
			'이샘플, 박예시, 최견본님이 눌렀어요'
		);
	});

	test('formats five names without shortening the list', () => {
		const names = ['이샘플1', '이샘플2', '이샘플3', '이샘플4', '이샘플5'];
		expect(reactionPeopleLabel(names, koreanTemplate)).toBe('이샘플1, 이샘플2, 이샘플3, 이샘플4, 이샘플5님이 눌렀어요');
	});

	test('formats an empty list as an empty string', () => {
		expect(reactionPeopleLabel([], koreanTemplate)).toBe('');
	});

	test('substitutes English templates the same way', () => {
		expect(reactionPeopleLabel(['Person One'], englishTemplate)).toBe('Person One reacted');
		const names = ['Person One', 'Person Two', 'Person Three', 'Person Four', 'Person Five'];
		expect(reactionPeopleLabel(names, englishTemplate)).toBe(
			'Person One, Person Two, Person Three, Person Four, Person Five reacted'
		);
	});
});

describe('what a reaction row looks like the moment the reader touches it', () => {
	const reader = { id: 'member:reader', name: '이샘플' };
	const other = { id: 'member:other', name: '박예시' };

	test('an emoji nobody used yet starts a reaction of one, marked as the reader’s', () => {
		const after = reactionsAfter([], { value: '👍', glyph: '👍', isAdding: true, person: reader });

		expect(after).toEqual([{ emoji: '👍', value: '👍', count: 1, reactedByMe: true, people: [reader] }]);
	});

	test('joining a reaction counts the reader in without starting a second one', () => {
		const before = [{ emoji: '😢', value: 'cry', count: 1, reactedByMe: false, people: [other] }];

		const after = reactionsAfter(before, { value: 'cry', glyph: '😢', isAdding: true, person: reader });

		expect(after).toEqual([{ emoji: '😢', value: 'cry', count: 2, reactedByMe: true, people: [other, reader] }]);
	});

	test('taking back the only vote removes the reaction instead of leaving a zero', () => {
		const before = [{ emoji: '👍', value: '👍', count: 1, reactedByMe: true, people: [reader] }];

		expect(reactionsAfter(before, { value: '👍', glyph: '👍', isAdding: false, person: reader })).toEqual([]);
	});

	test('taking back one of several leaves the others counted', () => {
		const before = [{ emoji: '👍', value: '👍', count: 2, reactedByMe: true, people: [other, reader] }];

		const after = reactionsAfter(before, { value: '👍', glyph: '👍', isAdding: false, person: reader });

		expect(after).toEqual([{ emoji: '👍', value: '👍', count: 1, reactedByMe: false, people: [other] }]);
	});

	test('adding what the reader already added changes nothing', () => {
		const before = [{ emoji: '👍', value: '👍', count: 1, reactedByMe: true, people: [reader] }];

		expect(reactionsAfter(before, { value: '👍', glyph: '👍', isAdding: true, person: reader })).toBe(before);
	});
});

describe('a reaction that arrives without the value it was sent with', () => {
	test('is known by its emoji, so two of them on one message stay two', () => {
		const sent = [
			{ emoji: '👍', count: 2 },
			{ emoji: '🎉', count: 1 }
		];

		expect(reactionsWithValues(sent)).toEqual([
			{ emoji: '👍', value: '👍', count: 2 },
			{ emoji: '🎉', value: '🎉', count: 1 }
		]);
	});

	test('keeps the value the messenger did send', () => {
		expect(reactionsWithValues([{ emoji: '😢', value: 'cry', count: 1 }])).toEqual([{ emoji: '😢', value: 'cry', count: 1 }]);
	});

	test('leaves a message without reactions without them', () => {
		expect(reactionsWithValues(undefined)).toBeUndefined();
	});
});
