import { afterEach, describe, expect, test } from 'bun:test';
import { holdBackNotificationsFor, messageNotificationTag } from '$lib/native-shell/open-conversation';
import { tellingOf } from '../../../host/relay/arrived.ts';

describe('the tag the shell holds back while a conversation is open', () => {
	test('is the tag the relay puts on that conversation\'s message notifications', () => {
		const told = tellingOf(
			{
				conversationID: 'channel-a',
				messageID: 'message-1',
				authorExternalID: 'author-1',
				authorName: '이샘플',
				recipientExternalIDs: ['reader-1'],
				preview: '보냈어요'
			},
			'이샘플'
		);
		expect(messageNotificationTag('channel-a')).toBe(told.tag);
	});
});

type Invocation = { command: string; args: unknown };

const originalWindow = globalThis.window;
const originalIsTauri = Reflect.get(globalThis, 'isTauri');

function insideTauri(platform: string, invocations: Invocation[]): void {
	Object.defineProperty(globalThis, 'window', {
		value: {
			__TAURI_INTERNALS__: {
				invoke: async (command: string, args: unknown) => {
					invocations.push({ command, args });
				}
			},
			__TAURI_OS_PLUGIN_INTERNALS__: { platform }
		},
		configurable: true
	});
	Reflect.set(globalThis, 'isTauri', true);
}

afterEach(() => {
	Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
	Reflect.set(globalThis, 'isTauri', originalIsTauri);
});

describe('holding back the open conversation in the Tauri app', () => {
	test('names the open conversation to the iPhone, and nothing once it closes', async () => {
		const invocations: Invocation[] = [];
		insideTauri('ios', invocations);
		await holdBackNotificationsFor('conversation-sample');
		await holdBackNotificationsFor(undefined);
		expect(invocations).toEqual([
			{ command: 'plugin:foreground-notifications|hold_back', args: { tag: 'message:conversation-sample' } },
			{ command: 'plugin:foreground-notifications|hold_back', args: { tag: '' } }
		]);
	});

	test('asks nothing of a platform that posts its own notifications', async () => {
		const invocations: Invocation[] = [];
		insideTauri('android', invocations);
		await holdBackNotificationsFor('conversation-sample');
		insideTauri('macos', invocations);
		await holdBackNotificationsFor('conversation-sample');
		expect(invocations).toEqual([]);
	});
});
