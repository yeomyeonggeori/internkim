import { describe, expect, test } from 'bun:test';
import { canEditMessage, editableTextOf, saveEditing } from '../../../src/lib/components/channel/message-edit';
import type { ChannelMessage } from '../../../src/lib/components/channel/channel-api';

const sender = { id: 'person-reader', name: '이샘플' };

function message(overrides: Partial<ChannelMessage> = {}): ChannelMessage {
	return { id: 'message-1', sender, text: '오타가 있는 문장', sentAt: '2026-09-28T01:00:00Z', ...overrides };
}

describe('which message can be edited', () => {
	test("only the reader's own settled message with text", () => {
		expect(canEditMessage(message(), true)).toBe(true);
		expect(canEditMessage(message(), false)).toBe(false);
		expect(canEditMessage(message({ id: 'pending-3' }), true)).toBe(false);
		expect(canEditMessage(message({ isError: true }), true)).toBe(false);
	});

	test('a message that is only a picture has no text to edit', () => {
		const picture = message({
			text: '![](https://files.example.com/a.png)',
			attachments: [{ kind: 'image', url: 'https://files.example.com/a.png' }]
		});
		expect(editableTextOf(picture)).toBe('');
		expect(canEditMessage(picture, true)).toBe(false);
	});
});

describe('saving an edit', () => {
	test('saves the trimmed text and reports whether the messenger took it', async () => {
		const saved: string[] = [];
		const refuse = async (messageID: string, text: string) => (saved.push(`${messageID}:${text}`), false);
		const accept = async (messageID: string, text: string) => (saved.push(`${messageID}:${text}`), true);
		expect(await saveEditing('message-1', '오타가 있는 문장', ' 고친 문장 ', refuse)).toBe('failed');
		expect(await saveEditing('message-1', '오타가 있는 문장', ' 고친 문장 ', accept)).toBe('saved');
		expect(saved).toEqual(['message-1:고친 문장', 'message-1:고친 문장']);
	});

	test('an unchanged or emptied edit ends without asking the messenger', async () => {
		let isAsked = false;
		const persist = async () => ((isAsked = true), true);
		expect(await saveEditing('message-1', '오타가 있는 문장', ' 오타가 있는 문장 ', persist)).toBe('unchanged');
		expect(await saveEditing('message-1', '오타가 있는 문장', '   ', persist)).toBe('unchanged');
		expect(isAsked).toBe(false);
	});
});
