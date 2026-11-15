import { describe, expect, test } from 'bun:test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
	BrowserHandoffs,
	handoffEndedEventKind,
	handoffFinishCapability,
	handoffInputCapability,
	handoffWatchCapability,
	type BrowserHandoffSettings
} from './browser-handoff';
import type { EndedHandoff } from './acp-session';
import { handoffFrameEventKind, type DevtoolsConnection } from './browser-handoff-stream';
import type { DevtoolsEvent } from './devtools-page';

type SentCommand = { method: string; params: Record<string, unknown> };

class FakePage implements DevtoolsConnection {
	isOpen = true;
	readonly sent: SentCommand[] = [];

	async send(method: string, params: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
		this.sent.push({ method, params });
		if (method === 'Page.captureScreenshot') return { data: 'first-frame' };
		if (method === 'Runtime.evaluate' && String(params.expression).includes('getBoundingClientRect')) return { result: { value: '[]' } };
		if (method === 'Runtime.evaluate') return { result: { value: JSON.stringify({ url: 'https://portal.example.com/login', title: 'Sign in' }) } };
		return {};
	}

	onEvent(_listener: (event: DevtoolsEvent) => void): void {}

	whenClosed(): void {}

	close(): void {
		this.isOpen = false;
	}

	methods(): string[] {
		return this.sent.map((command) => command.method);
	}
}

const requester = { memberID: 'member-sample', email: 'sample@example.com' };
const stranger = { memberID: 'member-other', email: 'other@example.com' };

type Delivered = { event: Record<string, unknown>; memberID: string };

function handoffsFixture(overrides: Partial<BrowserHandoffSettings> = {}) {
	const delivered: Delivered[] = [];
	const reported: string[] = [];
	const page = new FakePage();
	const settings: BrowserHandoffSettings = {
		appURL: 'https://intern.example.com/',
		filePath: join(mkdtempSync(join(tmpdir(), 'handoffs-')), 'browser-handoffs.json'),
		openPage: async () => page,
		deliver: (event, memberID) => delivered.push({ event, memberID }),
		emailOfMember: async (memberID) => [requester, stranger].find((member) => member.memberID === memberID)?.email ?? null,
		report: (line) => reported.push(line),
		...overrides
	};
	return { handoffs: new BrowserHandoffs(settings), settings, delivered, reported, page };
}

function registration(expiresInMilliseconds = 15 * 60_000) {
	return {
		handoffID: 'handoff-1',
		requesterEmail: 'Sample@example.com',
		devtoolsURL: 'http://127.0.0.1:9230',
		signInOrigin: 'https://portal.example.com',
		expiresAt: new Date(Date.now() + expiresInMilliseconds).toISOString()
	};
}

async function eventually(isTrue: () => boolean): Promise<void> {
	const deadline = Date.now() + 2_000;
	while (!isTrue() && Date.now() < deadline) await Bun.sleep(5);
}

const claimingSessionID = 'session-1';

async function openedHandoff(overrides: Partial<BrowserHandoffSettings> = {}) {
	const ended: EndedHandoff[] = [];
	const fixture = handoffsFixture({ whenEnded: (handoff) => ended.push(handoff), ...overrides });
	expect((await fixture.handoffs.register(registration())).status).toBe(201);
	expect(await fixture.handoffs.claim('handoff-1', requester.email, claimingSessionID)).toBe('open');
	return { ...fixture, ended };
}

describe('BrowserHandoffs', () => {
	test('a hand-over waits until its question claims it and is named by one link', async () => {
		const { handoffs } = handoffsFixture();

		await handoffs.register(registration());

		expect(handoffs.statusOf('handoff-1')).toEqual({ status: 200, body: { status: 'waiting', signInOrigin: 'https://portal.example.com' } });
		expect(handoffs.linkOf('handoff-1')).toBe('https://intern.example.com/handoff/handoff-1');
		expect(await handoffs.claim('handoff-1', requester.email, claimingSessionID)).toBe('open');
		expect(handoffs.statusOf('handoff-1').body).toEqual({ status: 'open', signInOrigin: 'https://portal.example.com' });
	});

	test('a question asked in someone else’s session cannot open the hand-over', async () => {
		const { handoffs } = handoffsFixture();
		await handoffs.register(registration());

		expect(await handoffs.claim('handoff-1', stranger.email, claimingSessionID)).toBe('refused');
		expect(handoffs.statusOf('handoff-1').body).toEqual({ status: 'waiting', signInOrigin: 'https://portal.example.com' });
	});

	test('only the requester sees or drives the handed-over browser', async () => {
		const { handoffs, delivered, page } = await openedHandoff();

		for (const capability of [handoffWatchCapability, handoffInputCapability, handoffFinishCapability]) {
			const refused = await handoffs.serve(capability, { handoffID: 'handoff-1', inputs: [{ type: 'text', text: 'x' }] }, stranger.memberID);
			expect(refused.status).toBe(403);
		}
		expect(page.sent).toEqual([]);
		expect(delivered).toEqual([]);

		const watched = await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1', viewport: { width: 390, height: 700 } }, requester.memberID);
		expect(watched.status).toBe(200);
		await Bun.sleep(10);
		expect(page.methods()).toContain('Page.startScreencast');
		expect(delivered.every((delivery) => delivery.memberID === requester.memberID)).toBe(true);
		expect(delivered.some((delivery) => delivery.event.kind === handoffFrameEventKind)).toBe(true);
	});

	test('what the requester types goes to the page and nowhere else', async () => {
		const { handoffs, page, reported, settings } = await openedHandoff();
		const typed = 'correct-horse-battery-staple';

		const taken = await handoffs.serve(handoffInputCapability, { handoffID: 'handoff-1', inputs: [{ type: 'text', text: typed }] }, requester.memberID);
		await Bun.sleep(10);
		await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'done' }, requester.memberID);

		expect(taken).toEqual({ status: 200, body: { accepted: 1 } });
		expect(page.sent).toContainEqual({ method: 'Input.insertText', params: { text: typed } });
		expect(reported.join('\n')).not.toContain(typed);
		expect(await Bun.file(settings.filePath).text()).not.toContain(typed);
	});

	test('finishing hands the browser back once, tells the page and reports the end to the session that claimed it', async () => {
		const { handoffs, delivered, page, ended } = await openedHandoff();
		await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, requester.memberID);

		const finished = await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'done' }, requester.memberID);
		const again = await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'done' }, requester.memberID);

		expect(finished).toEqual({ status: 200, body: { status: 'done' } });
		expect(again.status).toBe(404);
		expect(ended).toEqual([{ handoffID: 'handoff-1', sessionID: claimingSessionID, isHandedBack: true }]);
		expect(delivered).toContainEqual({ event: { kind: handoffEndedEventKind, handoffID: 'handoff-1', status: 'done' }, memberID: requester.memberID });
		await Bun.sleep(10);
		expect(page.methods()).toContain('Emulation.clearDeviceMetricsOverride');
		expect(page.isOpen).toBe(false);
	});

	test('stopping without finishing is told as abandoned', async () => {
		const { handoffs, ended } = await openedHandoff();

		await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'abandoned' }, requester.memberID);

		expect(handoffs.statusOf('handoff-1').body).toEqual({ status: 'abandoned', signInOrigin: 'https://portal.example.com' });
		expect(ended).toEqual([{ handoffID: 'handoff-1', sessionID: claimingSessionID, isHandedBack: false }]);
	});

	test('a hand-over nobody finishes expires and still reports its end as not handed back', async () => {
		const ended: EndedHandoff[] = [];
		const fixture = handoffsFixture({ whenEnded: (handoff) => ended.push(handoff) });
		const registering = fixture.handoffs.register(registration(30));
		const claiming = fixture.handoffs.claim('handoff-1', requester.email, claimingSessionID);
		expect(await claiming).toBe('open');
		await registering;

		await eventually(() => ended.length > 0);

		expect(fixture.handoffs.statusOf('handoff-1').body).toEqual({ status: 'expired', signInOrigin: 'https://portal.example.com' });
		expect(ended).toEqual([{ handoffID: 'handoff-1', sessionID: claimingSessionID, isHandedBack: false }]);
	});

	test('a hand-over no question ever claims expires so the browser is not left locked', async () => {
		const { handoffs } = handoffsFixture({ unclaimedMilliseconds: 20 });
		await handoffs.register(registration());

		await eventually(() => JSON.stringify(handoffs.statusOf('handoff-1').body).includes('expired'));

		expect(handoffs.statusOf('handoff-1').body).toEqual({ status: 'expired', signInOrigin: 'https://portal.example.com' });
	});

	test('a hand-over no session claimed ends without a report, since no run is parked on it', async () => {
		const ended: EndedHandoff[] = [];
		const { handoffs } = handoffsFixture({ unclaimedMilliseconds: 20, whenEnded: (handoff) => ended.push(handoff) });
		await handoffs.register(registration());

		await eventually(() => JSON.stringify(handoffs.statusOf('handoff-1').body).includes('expired'));

		expect(ended).toEqual([]);
	});

	test('a hand-over finished after a relay restart still reports its end to the session that claimed it', async () => {
		const first = await openedHandoff();
		const endedAfterTheRestart: EndedHandoff[] = [];
		const restarted = new BrowserHandoffs({ ...first.settings, whenEnded: (handoff) => endedAfterTheRestart.push(handoff) });
		await restarted.restore();

		await restarted.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'done' }, requester.memberID);

		expect(endedAfterTheRestart).toEqual([{ handoffID: 'handoff-1', sessionID: claimingSessionID, isHandedBack: true }]);
		expect(first.ended).toEqual([]);
		first.handoffs.close();
		restarted.close();
	});

	test('a hand-over closed unfinished ends as abandoned, tells the page, and reports nothing to the session', async () => {
		const { handoffs, delivered, ended, page } = await openedHandoff();
		await handoffs.serve(handoffWatchCapability, { handoffID: 'handoff-1' }, requester.memberID);

		await handoffs.closeUnfinished('handoff-1');
		await handoffs.closeUnfinished('handoff-nobody-registered');

		expect(handoffs.statusOf('handoff-1').body).toEqual({ status: 'abandoned', signInOrigin: 'https://portal.example.com' });
		expect(delivered).toContainEqual({ event: { kind: handoffEndedEventKind, handoffID: 'handoff-1', status: 'abandoned' }, memberID: requester.memberID });
		expect(ended).toEqual([]);
		await Bun.sleep(10);
		expect(page.isOpen).toBe(false);
	});

	test('an open hand-over survives a relay restart', async () => {
		const first = await openedHandoff();
		const restarted = new BrowserHandoffs(first.settings);

		await restarted.restore();

		expect(restarted.statusOf('handoff-1').body).toEqual({ status: 'open', signInOrigin: 'https://portal.example.com' });
		expect(await restarted.claim('handoff-1', requester.email, claimingSessionID)).toBe('open');
		first.handoffs.close();
		restarted.close();
	});

	test('a hand-over that ended before its question was put again is claimed as ended', async () => {
		const { handoffs } = await openedHandoff();
		await handoffs.serve(handoffFinishCapability, { handoffID: 'handoff-1', outcome: 'done' }, requester.memberID);

		expect(await handoffs.claim('handoff-1', requester.email, claimingSessionID)).toBe('ended');
		expect(await handoffs.claim('handoff-unknown', requester.email, claimingSessionID)).toBe('ended');
	});

	test('a registration naming a browser off this computer is refused', async () => {
		const { handoffs } = handoffsFixture();

		const refused = await handoffs.register({ ...registration(), devtoolsURL: 'http://browser.example.com:9230' });

		expect(refused.status).toBe(400);
	});
});
