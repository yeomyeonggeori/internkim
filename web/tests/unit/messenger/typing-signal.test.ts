import { describe, expect, test } from 'bun:test';
import {
	activityLabel,
	isTimeToAnnounceTyping,
	typersAfterSignal,
	typersStillTyping,
	typingLabel
} from '$lib/messenger/typing-signal';
import { channelText } from '$lib/i18n/channel-text';

const heardAt = Date.parse('2026-09-28T01:00:00Z');

describe('typersAfterSignal', () => {
	test('a person who keeps typing is one typer, heard at the latest signal', () => {
		const once = typersAfterSignal([], 'person-1', heardAt);
		const twice = typersAfterSignal(once, 'person-1', heardAt + 3_000);

		expect(twice).toEqual([{ externalID: 'person-1', heardAt: heardAt + 3_000 }]);
	});

	test('several people typing at once are kept in the order they started', () => {
		const typers = typersAfterSignal(typersAfterSignal([], 'person-1', heardAt), 'person-2', heardAt + 500);

		expect(typers.map((typer) => typer.externalID)).toEqual(['person-1', 'person-2']);
	});
});

describe('typersStillTyping', () => {
	const typers = [{ externalID: 'person-1', heardAt }];

	test('a typer is shown until eight seconds pass with no new signal', () => {
		expect(typersStillTyping(typers, [], heardAt + 7_999)).toEqual(typers);
		expect(typersStillTyping(typers, [], heardAt + 8_000)).toEqual([]);
	});

	test('a typer whose message arrived is no longer typing', () => {
		const sent = [{ senderExternalID: 'person-1', sentAt: '2026-09-28T01:00:01Z' }];

		expect(typersStillTyping(typers, sent, heardAt + 1_500)).toEqual([]);
	});

	test('a message stamped in the second before the signal still ends it', () => {
		const sent = [{ senderExternalID: 'person-1', sentAt: '2026-09-28T00:59:59Z' }];

		expect(typersStillTyping(typers, sent, heardAt + 500)).toEqual([]);
	});

	test('an older message, or someone else\'s, does not end it', () => {
		const sent = [
			{ senderExternalID: 'person-1', sentAt: '2026-09-28T00:59:00Z' },
			{ senderExternalID: 'person-2', sentAt: '2026-09-28T01:00:01Z' }
		];

		expect(typersStillTyping(typers, sent, heardAt + 1_500)).toEqual(typers);
	});
});

describe('isTimeToAnnounceTyping', () => {
	test('typing is announced at once, then at most every three seconds', () => {
		expect(isTimeToAnnounceTyping(null, heardAt)).toBe(true);
		expect(isTimeToAnnounceTyping(heardAt, heardAt + 2_999)).toBe(false);
		expect(isTimeToAnnounceTyping(heardAt, heardAt + 3_000)).toBe(true);
	});
});

describe('typingLabel', () => {
	test('a direct conversation names nobody', () => {
		expect(typingLabel(['박예시'], true, channelText.ko)).toBe('입력 중이에요');
	});

	test('a channel names up to two people and counts the rest', () => {
		expect(typingLabel(['박예시'], false, channelText.ko)).toBe('박예시님이 입력 중이에요');
		expect(typingLabel(['박예시', '최견본'], false, channelText.ko)).toBe('박예시, 최견본님이 입력 중이에요');
		expect(typingLabel(['박예시', '최견본', '이샘플'], false, channelText.ko)).toBe(
			'박예시, 최견본 외 1명이 입력 중이에요'
		);
	});

	test('reads the same in English', () => {
		expect(typingLabel(['박예시', '최견본'], false, channelText.en)).toBe('박예시 and 최견본 are typing…');
		expect(typingLabel(['박예시', '최견본', '이샘플'], false, channelText.en)).toBe(
			'박예시, 최견본 and 1 more are typing…'
		);
	});

	test('nobody typing is no label', () => {
		expect(typingLabel([], false, channelText.ko)).toBe('');
	});
});

describe('activityLabel', () => {
	const names = new Map([
		['person-1', '박예시'],
		['person-2', '최견본']
	]);
	const nameOf = (externalID: string) => names.get(externalID);

	test('a channel shows who is typing ahead of the agent working', () => {
		const activity = { typerExternalIDs: ['person-1'], nameOf, isGroup: true, isAgentWorking: true };

		expect(activityLabel(activity, channelText.ko)).toBe('박예시님이 입력 중이에요');
	});

	test('a channel falls back to the agent working when nobody is typing', () => {
		const activity = { typerExternalIDs: [], nameOf, isGroup: true, isAgentWorking: true };

		expect(activityLabel(activity, channelText.ko)).toBe('김인턴이 작업 중이에요');
	});

	test('a channel leaves out a typer it cannot name', () => {
		const activity = { typerExternalIDs: ['person-9', 'person-2'], nameOf, isGroup: true, isAgentWorking: false };

		expect(activityLabel(activity, channelText.ko)).toBe('최견본님이 입력 중이에요');
	});

	test('a direct conversation shows the agent working ahead of typing', () => {
		const activity = { typerExternalIDs: ['person-1'], nameOf, isGroup: false, isAgentWorking: true };

		expect(activityLabel(activity, channelText.ko)).toBe('김인턴이 작업 중이에요');
	});

	test('a direct conversation shows typing without needing a name', () => {
		const activity = { typerExternalIDs: ['person-9'], nameOf, isGroup: false, isAgentWorking: false };

		expect(activityLabel(activity, channelText.ko)).toBe('입력 중이에요');
	});

	test('nothing happening is no label', () => {
		const activity = { typerExternalIDs: [], nameOf, isGroup: true, isAgentWorking: false };

		expect(activityLabel(activity, channelText.ko)).toBe('');
	});
});
