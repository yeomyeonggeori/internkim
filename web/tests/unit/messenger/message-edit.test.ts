import { describe, expect, test } from 'bun:test';
import {
	canEditMessage,
	composerEditing,
	editableTextOf,
	saveEditing,
	startEditing,
	type EditingMessage
} from '../../../src/lib/components/channel/message-edit';
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

describe('editing in the composer', () => {
	test('keeps the draft that was there before the first edit, across a second edit', () => {
		const first = startEditing(message(), '보내던 초안', null);
		expect(first).toEqual({ messageID: 'message-1', originalText: '오타가 있는 문장', draftBeforeEditing: '보내던 초안' });
		const second = startEditing(message({ id: 'message-2', text: '다른 문장' }), '오타가 있는 문장', first);
		expect(second).toEqual({ messageID: 'message-2', originalText: '다른 문장', draftBeforeEditing: '보내던 초안' });
	});

	test('saves the trimmed text and reports whether the messenger took it', async () => {
		const editing = startEditing(message(), '', null);
		const saved: string[] = [];
		const refuse = async (messageID: string, text: string) => (saved.push(`${messageID}:${text}`), false);
		const accept = async (messageID: string, text: string) => (saved.push(`${messageID}:${text}`), true);
		expect(await saveEditing(editing, ' 고친 문장 ', refuse)).toBe('failed');
		expect(await saveEditing(editing, ' 고친 문장 ', accept)).toBe('saved');
		expect(saved).toEqual(['message-1:고친 문장', 'message-1:고친 문장']);
	});

	test('a composer gets its unsent draft back after a save, and keeps the edit open after a failure', async () => {
		let text = '보내던 초안';
		let editing: EditingMessage | null = null;
		let focusCount = 0;
		const composer = composerEditing({
			text: () => text,
			setText: (written) => (text = written),
			editing: () => editing,
			setEditing: (next) => (editing = next),
			focus: () => (focusCount += 1)
		});

		composer.begin(message());
		expect(text).toBe('오타가 있는 문장');
		expect(focusCount).toBe(1);

		text = '고친 문장';
		await composer.save(async () => false);
		expect(text).toBe('고친 문장');
		expect(editing).not.toBeNull();

		await composer.save(async () => true);
		expect(text).toBe('보내던 초안');
		expect(editing).toBeNull();

		composer.begin(message());
		composer.cancel();
		expect(text).toBe('보내던 초안');
		expect(editing).toBeNull();
	});

	test('an unchanged or emptied edit ends without asking the messenger', async () => {
		const editing = startEditing(message(), '', null);
		let isAsked = false;
		const persist = async () => ((isAsked = true), true);
		expect(await saveEditing(editing, ' 오타가 있는 문장 ', persist)).toBe('unchanged');
		expect(await saveEditing(editing, '   ', persist)).toBe('unchanged');
		expect(isAsked).toBe(false);
	});
});
