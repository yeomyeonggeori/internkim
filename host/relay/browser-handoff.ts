import type { Addressing, Requester } from './acp-session';
import { devtoolsCommandsFor, readHandoffInputs, type DevtoolsCommand } from './browser-handoff-input';
import type { DevtoolsEvent } from './devtools-page';

export const handoffWatchCapability = 'person.browser.handoff.watch';
export const handoffInputCapability = 'person.browser.handoff.input';
export const handoffFinishCapability = 'person.browser.handoff.finish';
export const handoffFrameEventKind = 'browser.handoff.frame';
export const handoffEndedEventKind = 'browser.handoff.ended';

const handoffCapabilities = new Set([handoffWatchCapability, handoffInputCapability, handoffFinishCapability]);

export function isBrowserHandoffCapability(capability: string): boolean {
	return handoffCapabilities.has(capability);
}

export type DevtoolsConnection = {
	readonly isOpen: boolean;
	send: (method: string, params?: Record<string, unknown>) => Promise<Record<string, unknown>>;
	onEvent: (listener: (event: DevtoolsEvent) => void) => void;
	whenClosed: (listener: () => void) => void;
	close: () => void;
};

export type HandoffRequest = {
	message: string;
	requester: Requester;
	addressing: Addressing;
};

export type BegunHandoff = {
	handoffID: string;
	openURL: string;
	expiresAt: string;
};

export type HandoffOutcome = 'completed' | 'abandoned' | 'expired';

export type BrowserHandoffSettings = {
	appURL: string;
	openPage: () => Promise<DevtoolsConnection>;
	deliver: (event: Record<string, unknown>, memberID: string) => void;
	resumeConversation: (inbound: Record<string, unknown>) => Promise<void>;
	emailOfMember: (memberID: string) => Promise<string | null>;
	now?: () => number;
	newHandoffID?: () => string;
	lifetimeMilliseconds?: number;
	report?: (line: string) => void;
};

type Served = { status: number; body: unknown };

type WaitingHandoff = HandoffRequest & {
	handoffID: string;
	expiresAt: number;
	expiry: ReturnType<typeof setTimeout>;
	stream: HandoffStream | null;
};

const handoffLifetimeMilliseconds = 15 * 60_000;
export const viewport = { width: 1280, height: 800 };
const watchLeaseMilliseconds = 45_000;
const captureDelaysAfterInputMilliseconds = [80, 500];
const jpegQuality = 60;

export function readHandoffRequest(offered: unknown): HandoffRequest | null {
	const held = recordOf(offered);
	const requester = recordOf(held.requester);
	const addressing = recordOf(held.addressing);
	const email = textOf(requester.email).toLowerCase();
	const platform = textOf(addressing.platform);
	const conversationID = textOf(addressing.conversationID);
	if (!email || !platform || !conversationID) return null;
	return {
		message: textOf(held.message),
		requester: { email, ...optionalText('name', requester.name) },
		addressing: {
			platform,
			conversationID,
			...optionalText('conversationType', addressing.conversationType),
			...optionalText('replyTargetID', addressing.replyTargetID),
			...optionalText('responseLanguage', addressing.responseLanguage),
			...(addressing.isThread === true ? { isThread: true } : {})
		}
	};
}

export class BrowserHandoffs {
	private readonly waiting = new Map<string, WaitingHandoff>();
	private readonly now: () => number;
	private readonly report: (line: string) => void;

	constructor(private readonly settings: BrowserHandoffSettings) {
		this.now = settings.now ?? Date.now;
		this.report = settings.report ?? ((line) => console.log(line));
	}

	begin(request: HandoffRequest): BegunHandoff {
		const handoffID = this.settings.newHandoffID?.() ?? crypto.randomUUID();
		const lifetime = this.settings.lifetimeMilliseconds ?? handoffLifetimeMilliseconds;
		const expiresAt = this.now() + lifetime;
		const expiry = setTimeout(() => this.expire(handoffID), lifetime);
		this.waiting.set(handoffID, { ...request, handoffID, expiresAt, expiry, stream: null });
		return {
			handoffID,
			openURL: `${this.settings.appURL.replace(/\/+$/, '')}/handoff/${encodeURIComponent(handoffID)}`,
			expiresAt: new Date(expiresAt).toISOString()
		};
	}

	async serve(capability: string, body: Record<string, unknown>, memberID: string): Promise<Served> {
		const handoff = this.waiting.get(textOf(body.handoffID));
		if (!handoff) return { status: 404, body: { error: 'this browser handoff is no longer waiting' } };
		const requesterEmail = (await this.settings.emailOfMember(memberID))?.toLowerCase();
		if (requesterEmail !== handoff.requester.email) {
			return { status: 403, body: { error: 'this browser handoff was handed to someone else' } };
		}
		if (capability === handoffWatchCapability) return this.watch(handoff, memberID);
		if (capability === handoffInputCapability) return this.takeInputs(handoff, memberID, body.inputs);
		return this.finish(handoff, body.outcome);
	}

	close(): void {
		for (const handoff of this.waiting.values()) {
			clearTimeout(handoff.expiry);
			handoff.stream?.stop();
		}
		this.waiting.clear();
	}

	private async watch(handoff: WaitingHandoff, memberID: string): Promise<Served> {
		const stream = await this.streamFor(handoff, memberID);
		stream.renewLease();
		stream.captureNow();
		return {
			status: 200,
			body: {
				handoffID: handoff.handoffID,
				message: handoff.message,
				expiresAt: new Date(handoff.expiresAt).toISOString(),
				viewport
			}
		};
	}

	private async takeInputs(handoff: WaitingHandoff, memberID: string, offered: unknown): Promise<Served> {
		const inputs = readHandoffInputs(offered);
		if (!inputs) return { status: 400, body: { error: 'inputs must be a non-empty list of browser inputs' } };
		const stream = await this.streamFor(handoff, memberID);
		stream.renewLease();
		await stream.dispatch(inputs.flatMap(devtoolsCommandsFor));
		return { status: 200, body: { accepted: inputs.length } };
	}

	private async finish(handoff: WaitingHandoff, offered: unknown): Promise<Served> {
		const outcome: HandoffOutcome = offered === 'abandoned' ? 'abandoned' : 'completed';
		await this.end(handoff.handoffID, outcome);
		return { status: 200, body: { outcome } };
	}

	private async streamFor(handoff: WaitingHandoff, memberID: string): Promise<HandoffStream> {
		if (handoff.stream?.isLive && handoff.stream.memberID === memberID) return handoff.stream;
		handoff.stream?.stop();
		const stream = new HandoffStream({
			handoffID: handoff.handoffID,
			memberID,
			page: await this.settings.openPage(),
			deliver: this.settings.deliver,
			now: this.now,
			report: this.report
		});
		handoff.stream = stream;
		await stream.start();
		return stream;
	}

	private expire(handoffID: string): void {
		this.end(handoffID, 'expired').catch((refusal) =>
			this.report(`browser handoff ${handoffID} expired but the conversation was not resumed: ${String(refusal)}`)
		);
	}

	private async end(handoffID: string, outcome: HandoffOutcome): Promise<void> {
		const handoff = this.waiting.get(handoffID);
		if (!handoff) return;
		this.waiting.delete(handoffID);
		clearTimeout(handoff.expiry);
		const where = await this.whereTheBrowserIs(handoff);
		if (handoff.stream) {
			this.settings.deliver({ kind: handoffEndedEventKind, handoffID, outcome }, handoff.stream.memberID);
			handoff.stream.stop();
		}
		await this.settings.resumeConversation(resumingInboundOf(handoff, outcome, where));
		this.report(`browser handoff ${handoffID} ended ${outcome}`);
	}

	private async whereTheBrowserIs(handoff: WaitingHandoff): Promise<PageWhereabouts> {
		try {
			const page = handoff.stream?.isLive ? handoff.stream.page : await this.settings.openPage();
			const evaluated = await page.send('Runtime.evaluate', {
				expression: 'JSON.stringify({ url: location.href, title: document.title })',
				returnByValue: true
			});
			if (!handoff.stream?.isLive) page.close();
			return whereaboutsOf(evaluated);
		} catch (refusal) {
			this.report(`browser handoff ${handoff.handoffID} could not read the page: ${String(refusal)}`);
			return { url: '', title: '' };
		}
	}
}

type PageWhereabouts = { url: string; title: string };

function whereaboutsOf(evaluated: Record<string, unknown>): PageWhereabouts {
	const value = recordOf(evaluated.result).value;
	if (typeof value !== 'string') return { url: '', title: '' };
	const parsed = recordOf(JSON.parse(value));
	return { url: textOf(parsed.url), title: textOf(parsed.title) };
}

export function resumingInboundOf(
	handoff: HandoffRequest & { handoffID: string },
	outcome: HandoffOutcome,
	where: PageWhereabouts
): Record<string, unknown> {
	const { requester, addressing } = handoff;
	return {
		platform: addressing.platform,
		conversationID: addressing.conversationID,
		messageID: `browser-handoff:${handoff.handoffID}`,
		prompt: resumingPromptOf(handoff.message, outcome, where),
		...(addressing.replyTargetID ? { replyTargetID: addressing.replyTargetID } : {}),
		...(addressing.isThread ? { isThread: true } : {}),
		context: {
			sender: { email: requester.email, platform: addressing.platform, ...(requester.name ? { name: requester.name } : {}) },
			...(addressing.conversationType ? { conversationType: addressing.conversationType } : {}),
			...(addressing.responseLanguage ? { responseLanguage: addressing.responseLanguage } : {})
		}
	};
}

function resumingPromptOf(message: string, outcome: HandoffOutcome, where: PageWhereabouts): string {
	const asked = message ? ` for "${message}"` : '';
	const page = where.url ? ` The device browser is now at ${where.url}${where.title ? ` (${where.title})` : ''}.` : '';
	if (outcome === 'completed') {
		return `[browser handoff] I finished the browser step you handed me${asked}.${page} Take a browser_snapshot and continue the task.`;
	}
	if (outcome === 'abandoned') {
		return `[browser handoff] I closed the browser handoff${asked} without finishing it.${page} Ask me how to proceed instead of retrying the same step.`;
	}
	return `[browser handoff] Nobody finished the browser handoff${asked} before it expired.${page} Tell me it expired and ask whether to hand it over again.`;
}

type HandoffStreamSettings = {
	handoffID: string;
	memberID: string;
	page: DevtoolsConnection;
	deliver: (event: Record<string, unknown>, memberID: string) => void;
	now: () => number;
	report: (line: string) => void;
};

class HandoffStream {
	readonly memberID: string;
	readonly page: DevtoolsConnection;
	private leaseRenewedAt = 0;
	private leaseCheck: ReturnType<typeof setInterval> | null = null;
	private lastImage = '';
	private url = '';
	private isStopped = false;
	private isCapturing = false;

	constructor(private readonly settings: HandoffStreamSettings) {
		this.memberID = settings.memberID;
		this.page = settings.page;
	}

	get isLive(): boolean {
		return !this.isStopped && this.page.isOpen;
	}

	async start(): Promise<void> {
		this.page.onEvent((event) => this.onPageEvent(event));
		this.page.whenClosed(() => this.stop());
		await this.page.send('Page.enable');
		await this.page.send('Emulation.setDeviceMetricsOverride', { ...viewport, deviceScaleFactor: 1, mobile: false });
		await this.page.send('Page.startScreencast', { format: 'jpeg', quality: jpegQuality, maxWidth: viewport.width, maxHeight: viewport.height });
		this.leaseCheck = setInterval(() => this.stopWhenNobodyWatches(), watchLeaseMilliseconds / 3);
	}

	renewLease(): void {
		this.leaseRenewedAt = this.settings.now();
	}

	async dispatch(commands: DevtoolsCommand[]): Promise<void> {
		for (const command of commands) await this.page.send(command.method, command.params);
		for (const delay of captureDelaysAfterInputMilliseconds) setTimeout(() => this.captureNow(), delay);
	}

	captureNow(): void {
		if (this.isCapturing || !this.isLive) return;
		this.isCapturing = true;
		this.page
			.send('Page.captureScreenshot', { format: 'jpeg', quality: jpegQuality })
			.then((captured) => this.publish(textOf(captured.data)))
			.catch((refusal) => this.settings.report(`browser handoff ${this.settings.handoffID} capture failed: ${String(refusal)}`))
			.finally(() => {
				this.isCapturing = false;
			});
	}

	stop(): void {
		if (this.isStopped) return;
		this.isStopped = true;
		if (this.leaseCheck) clearInterval(this.leaseCheck);
		if (!this.page.isOpen) return;
		this.page
			.send('Page.stopScreencast')
			.catch(() => undefined)
			.finally(() => this.page.close());
	}

	private stopWhenNobodyWatches(): void {
		if (this.settings.now() - this.leaseRenewedAt > watchLeaseMilliseconds) this.stop();
	}

	private onPageEvent(event: DevtoolsEvent): void {
		if (event.method === 'Page.screencastFrame') {
			void this.page.send('Page.screencastFrameAck', { sessionId: event.params.sessionId }).catch(() => undefined);
			this.publish(textOf(event.params.data));
			return;
		}
		if (event.method === 'Page.frameNavigated') {
			const frame = recordOf(event.params.frame);
			if (frame.parentId === undefined) this.url = textOf(frame.url);
		}
	}

	private publish(image: string): void {
		if (!image || image === this.lastImage || !this.isLive) return;
		this.lastImage = image;
		this.settings.deliver(
			{ kind: handoffFrameEventKind, handoffID: this.settings.handoffID, image, ...viewport, url: this.url },
			this.memberID
		);
	}
}

function optionalText<Key extends string>(key: Key, offered: unknown): Partial<Record<Key, string>> {
	const text = textOf(offered);
	if (!text) return {};
	const entry: Partial<Record<Key, string>> = {};
	entry[key] = text;
	return entry;
}

function textOf(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}
