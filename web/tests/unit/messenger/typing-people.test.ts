import { describe, expect, test } from 'bun:test';
import { typingPeople } from '$lib/components/channel/typing-people';
import type { ChannelMessage, ChannelParticipant } from '$lib/components/channel/channel-api';

function messageFrom(sender: ChannelParticipant, id: string): ChannelMessage {
	return { id, sender, text: 'hello', sentAt: '2026-11-11T00:00:00Z' };
}

const firstSender: ChannelParticipant = {
	id: 'person-1',
	name: '이샘플',
	email: 'sample@example.com',
	avatarURL: 'https://example.com/old.png',
	externalID: 'U-sample'
};

describe('typingPeople', () => {
	test('draws a typer the way their latest message draws them', () => {
		const latest = { ...firstSender, avatarURL: 'https://example.com/new.png' };
		const messages = [messageFrom(firstSender, 'm-1'), messageFrom(latest, 'm-2')];

		expect(typingPeople(['U-sample'], messages, [])).toEqual([latest]);
	});

	test('falls back to the conversation member when the typer has written nothing loaded', () => {
		const participants = [{ externalID: 'U-example', name: '박예시', memberID: 'member-2' }];

		expect(typingPeople(['U-example'], [], participants)).toEqual([
			{ id: 'U-example', name: '박예시', avatarURL: undefined, memberID: 'member-2', externalID: 'U-example' }
		]);
	});

	test('keeps a typer nobody knows, so the dots still show', () => {
		expect(typingPeople(['U-unknown'], [], [])).toEqual([
			{ id: 'U-unknown', name: '', avatarURL: undefined, memberID: undefined, externalID: 'U-unknown' }
		]);
	});
});
