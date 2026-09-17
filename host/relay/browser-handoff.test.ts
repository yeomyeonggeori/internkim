import { afterEach, describe, expect, test } from 'bun:test';
import {
	BrowserHandoffs,
	handoffEndedEventKind,
	handoffFinishCapability,
	handoffFrameEventKind,
	handoffInputCapability,
	handoffWatchCapability,
	readHandoffRequest,
	type BrowserHandoffSettings,
	type DevtoolsConnection,
	type HandoffRequest
} from './browser-handoff';
import { readInboundMessage } from './inbound-message';
import type { DevtoolsEvent } from './devtools-page';

type SentCommand = { method: string; params: Record<string, unknown> };

class FakePage implements DevtoolsConnection {
	isOpen = true;
	readonly sent: SentCommand[] = [];
	private readonly listeners: ((event: DevtoolsEvent) => void)[] = [];
	screenshot = 'first-frame';

	async send(method: string, params: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
		this.sent.push({ method, params });
		if (method === 'Page.captureScreenshot') return { data: this.screenshot };
		if (method === 'Runtime.evaluate' && params.returnByValue) {
			return { result: { value: JSON.stringify({ url: 'https://example.com/signed-in', title: 'Signed in' }) } };
		}
		return {};
	}

	onEvent(listener: (event: DevtoolsEvent) => void): void {
		this.listeners.push(listener);
	}

	whenClosed(): void {}

	close(): void {
		this.isOpen = false;
	}

	emit(event: DevtoolsEvent): void {
		for (const listener of this.listeners) listener(event);
	}

	methods(): string[] {
		return this.sent.map((command) => command.method);
	}
}

const request: HandoffRequest = {
	message: 'Sign in to the tax office',
	requester: { email: 'sample@example.test', name: '이샘플' },
	addressing: { platform: 'buzz', conversationID: 'conversation-1', conversationType: 'direct', responseLanguage: 'ko' }
};

const openHandoffs: BrowserHandoffs[] = [];

afterEach(() => {
	for (const handoffs of openHandoffs.splice(0)) handoffs.close();
});

function handoffsWith(overrides: Partial<BrowserHandoffSettings> = {}) {
	const page = new FakePage();
	const delivered: { event: Record<string, unknown>; memberID: string }[] = [];
	const resumed: Record<string, unknown>[] = [];
	const handoffs = new BrowserHandoffs({
		appURL: 'https://intern.kim/',
		openPage: async () => page,
		deliver: (event, memberID) => delivered.push({ event, memberID }),
		resumeConversation: async (inbound) => {
			resumed.push(inbound);
		},
		emailOfMember: async (memberID) => (memberID === 'member-1' ? 'Sample@Example.test' : 'other@example.test'),
		newHandoffID: () => 'handoff-1',
		report: () => {},
		...overrides
	});
	openHandoffs.push(handoffs);
	return { handoffs, page, delivered, resumed };
}

const settle = () => new Promise((resolve) => setTimeout(resolve, 10));

describe('readHandoffRequest', () => {
	test('needs a requester and the conversation to resume', () => {
		expect(readHandoffRequest({ message: 'x', requester: { email: 'a@b.test' } })).toBeNull();
		expect(readHandoffRequest({ addressing: { platform: 'buzz', conversationID: 'c' } })).toBeNull();
		expect(
			readHandoffRequest({
				message: ' Sign in ',
				requester: { email: 'A@B.test' },
				addressing: { platform: 'buzz', conversationID: 'c', isThread: true }
			})
		).toEqual({
			message: 'Sign in',
			requester: { email: 'a@b.test' },
			addressing: { platform: 'buzz', conversationID: 'c', isThread: true }
		});
	});
});

describe('BrowserHandoffs', () => {
	test('a begun handoff is opened on the website by its own address', () => {
		const { handoffs } = handoffsWith({ now: () => Date.parse('2026-09-17T00:00:00Z') });

		expect(handoffs.begin(request)).toEqual({
			handoffID: 'handoff-1',
			openURL: 'https://intern.kim/handoff/handoff-1',
			expiresAt: '2026-09-17T00:15:00.000Z'
		});
	});

	test('only the requester may watch the browser', async () => {
		const { handoffs, page } = handoffsWith();
		handoffs.begin(request);

		const refused = await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, 'member-2');

		expect(refused.status).toBe(403);
		expect(page.sent).toEqual([]);
	});

	test('a handoff nobody began is not waiting', async () => {
		const { handoffs } = handoffsWith();

		const answered = await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-unknown' }, 'member-1');

		expect(answered.status).toBe(404);
	});

	test('watching starts the screencast and sends the current screen to the requester', async () => {
		const { handoffs, page, delivered } = handoffsWith();
		handoffs.begin(request);

		const watched = await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, 'member-1');
		await settle();

		expect(watched).toEqual({
			status: 200,
			body: {
				handoffID: 'handoff-1',
				message: 'Sign in to the tax office',
				expiresAt: expect.any(String),
				viewport: { width: 1280, height: 800 }
			}
		});
		expect(page.methods()).toEqual([
			'Page.enable',
			'Emulation.setDeviceMetricsOverride',
			'Page.startScreencast',
			'Page.captureScreenshot'
		]);
		expect(delivered).toEqual([
			{
				event: { kind: handoffFrameEventKind, handoffID: 'handoff-1', image: 'first-frame', width: 1280, height: 800, url: '' },
				memberID: 'member-1'
			}
		]);
	});

	test('a screencast frame is acknowledged and an unchanged frame is not sent twice', async () => {
		const { handoffs, page, delivered } = handoffsWith();
		handoffs.begin(request);
		await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, 'member-1');
		await settle();

		page.emit({ method: 'Page.frameNavigated', params: { frame: { url: 'https://example.com/login' } } });
		page.emit({ method: 'Page.screencastFrame', params: { sessionId: 7, data: 'first-frame' } });
		page.emit({ method: 'Page.screencastFrame', params: { sessionId: 8, data: 'second-frame' } });
		await settle();

		expect(page.sent.filter((command) => command.method === 'Page.screencastFrameAck')).toEqual([
			{ method: 'Page.screencastFrameAck', params: { sessionId: 7 } },
			{ method: 'Page.screencastFrameAck', params: { sessionId: 8 } }
		]);
		expect(delivered.map((delivery) => delivery.event.image)).toEqual(['first-frame', 'second-frame']);
		expect(delivered[1].event.url).toBe('https://example.com/login');
	});

	test('inputs reach the browser in order and the screen is captured again', async () => {
		const { handoffs, page } = handoffsWith();
		handoffs.begin(request);
		await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, 'member-1');

		const answered = await handoffs.serve(
			handoffInputCapability,
			{
				handoffID: 'handoff-1',
				inputs: [
					{ type: 'mouse', action: 'down', x: 1, y: 2, button: 'left', clickCount: 1 },
					{ type: 'text', text: '홍길동' }
				]
			},
			'member-1'
		);
		await new Promise((resolve) => setTimeout(resolve, 120));

		expect(answered).toEqual({ status: 200, body: { accepted: 2 } });
		const afterWatch = page.methods().slice(4);
		expect(afterWatch.slice(0, 2)).toEqual(['Input.dispatchMouseEvent', 'Input.insertText']);
		expect(afterWatch).toContain('Page.captureScreenshot');
	});

	test('inputs the relay does not understand are refused before reaching the browser', async () => {
		const { handoffs, page } = handoffsWith();
		handoffs.begin(request);

		const refused = await handoffs.serve(handoffInputCapability, { handoffID: 'handoff-1', inputs: [{ type: 'eval' }] }, 'member-1');

		expect(refused.status).toBe(400);
		expect(page.sent).toEqual([]);
	});

	test('finishing ends the handoff and resumes the conversation where the browser is', async () => {
		const { handoffs, page, delivered, resumed } = handoffsWith();
		handoffs.begin(request);
		await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, 'member-1');

		const finished = await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'completed' }, 'member-1');
		await settle();

		expect(finished).toEqual({ status: 200, body: { outcome: 'completed' } });
		expect(delivered.at(-1)).toEqual({
			event: { kind: handoffEndedEventKind, handoffID: 'handoff-1', outcome: 'completed' },
			memberID: 'member-1'
		});
		expect(page.methods()).toContain('Page.stopScreencast');
		expect(page.isOpen).toBe(false);
		expect(resumed).toHaveLength(1);
		const inbound = readInboundMessage(resumed[0]);
		expect(inbound?.key).toBe('buzz:conversation-1:browser-handoff:handoff-1');
		expect(inbound?.requester).toEqual({ email: 'sample@example.test', name: '이샘플' });
		expect(inbound?.addressing).toEqual({
			platform: 'buzz',
			conversationID: 'conversation-1',
			conversationType: 'direct',
			replyTargetID: undefined,
			isThread: false,
			responseLanguage: 'ko'
		});
		expect(inbound?.message).toContain('https://example.com/signed-in (Signed in)');
		expect(inbound?.message).toContain('browser_snapshot');

		const again = await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, 'member-1');
		expect(again.status).toBe(404);
	});

	test('a handoff closed without finishing tells the agent to ask rather than retry', async () => {
		const { handoffs, resumed } = handoffsWith();
		handoffs.begin(request);

		await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'abandoned' }, 'member-1');

		expect(readInboundMessage(resumed[0])?.message).toContain('without finishing it');
	});

	test('a handoff nobody finishes expires and resumes the conversation once', async () => {
		const { handoffs, resumed } = handoffsWith({ lifetimeMilliseconds: 20 });
		handoffs.begin(request);

		await new Promise((resolve) => setTimeout(resolve, 60));

		expect(resumed).toHaveLength(1);
		expect(readInboundMessage(resumed[0])?.message).toContain('before it expired');
	});
});
