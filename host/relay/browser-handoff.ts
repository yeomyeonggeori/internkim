import type { Addressing, Requester } from './acp-session';
import { devtoolsCommandsFor, readHandoffInputs, type DevtoolsCommand } from './browser-handoff-input';
import type { DevtoolsEvent } from './devtools-page';

export const handoffWatchCapability = 'person.browser.handoff.watch';
export const handoffInputCapability = 'person.browser.handoff.input';
export const handoffFinishCapability = 'person.browser.handoff.finish';
export const handoffFrameEventKind = 'browser.handoff.frame';
export const handoffEndedEventKind = 'browser.handoff.ended';
export const handoffTroubleEventKind = 'browser.handoff.trouble';

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
	devtoolsURL: string;
};

export type BegunHandoff = {
	handoffID: string;
	openURL: string;
	expiresAt: string;
};

export type HandoffOutcome = 'completed' | 'abandoned' | 'expired';

export type BrowserHandoffSettings = {
	appURL: string;
	openPage: (devtoolsURL: string) => Promise<DevtoolsConnection>;
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
	requesterMemberID: string | null;
	opening: Promise<HandoffStream> | null;
};

const handoffLifetimeMilliseconds = 15 * 60_000;
export type Viewport = { width: number; height: number };

export const defaultViewport: Viewport = { width: 1280, height: 800 };
const smallestViewportSide = 240;
const largestViewportSide = 2560;
const watchLeaseMilliseconds = 45_000;
const captureDelaysAfterInputMilliseconds = [80, 500];
const fieldReadingGapMilliseconds = 1_000;
const troubleReportGapMilliseconds = 5_000;
const jpegQuality = 60;

export function readHandoffRequest(offered: unknown): HandoffRequest | null {
	const held = recordOf(offered);
	const requester = recordOf(held.requester);
	const addressing = recordOf(held.addressing);
	const email = textOf(requester.email).toLowerCase();
	const platform = textOf(addressing.platform);
	const conversationID = textOf(addressing.conversationID);
	const devtoolsURL = loopbackDevtoolsURLOf(held.devtoolsURL);
	if (!email || !platform || !conversationID || !devtoolsURL) return null;
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
		},
		devtoolsURL
	};
}

function loopbackDevtoolsURLOf(offered: unknown): string {
	const text = textOf(offered);
	if (!URL.canParse(text)) return '';
	const address = new URL(text);
	const isLoopback = address.hostname === '127.0.0.1' || address.hostname === 'localhost';
	if (address.protocol !== 'http:' || !isLoopback || !address.port) return '';
	return address.origin;
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
		this.waiting.set(handoffID, { ...request, handoffID, expiresAt, expiry, stream: null, requesterMemberID: null, opening: null });
		return {
			handoffID,
			openURL: `${this.settings.appURL.replace(/\/+$/, '')}/handoff/${encodeURIComponent(handoffID)}`,
			expiresAt: new Date(expiresAt).toISOString()
		};
	}

	async serve(capability: string, body: Record<string, unknown>, memberID: string): Promise<Served> {
		const handoff = this.waiting.get(textOf(body.handoffID));
		if (!handoff) return { status: 404, body: { error: 'this browser handoff is no longer waiting' } };
		if (!(await this.isTheRequester(handoff, memberID))) {
			return { status: 403, body: { error: 'this browser handoff was handed to someone else' } };
		}
		if (capability === handoffWatchCapability) return this.watch(handoff, memberID, body.viewport);
		if (capability === handoffInputCapability) return this.takeInputs(handoff, memberID, body.inputs);
		return this.finish(handoff, body.outcome);
	}

	private async isTheRequester(handoff: WaitingHandoff, memberID: string): Promise<boolean> {
		if (handoff.requesterMemberID === memberID) return true;
		const email = (await this.settings.emailOfMember(memberID))?.toLowerCase();
		if (email !== handoff.requester.email) return false;
		handoff.requesterMemberID = memberID;
		return true;
	}

	close(): void {
		for (const handoff of this.waiting.values()) {
			clearTimeout(handoff.expiry);
			handoff.stream?.stop();
		}
		this.waiting.clear();
	}

	private async watch(handoff: WaitingHandoff, memberID: string, offeredViewport: unknown): Promise<Served> {
		const stream = await this.streamFor(handoff, memberID, readViewport(offeredViewport));
		stream.renewLease();
		stream.captureNow();
		return {
			status: 200,
			body: {
				handoffID: handoff.handoffID,
				message: handoff.message,
				expiresAt: new Date(handoff.expiresAt).toISOString(),
				viewport: stream.viewport
			}
		};
	}

	private async takeInputs(handoff: WaitingHandoff, memberID: string, offered: unknown): Promise<Served> {
		const inputs = readHandoffInputs(offered);
		if (!inputs) return { status: 400, body: { error: 'inputs must be a non-empty list of browser inputs' } };
		const stream = await this.streamFor(handoff, memberID, null);
		stream.renewLease();
		stream.take(inputs.flatMap(devtoolsCommandsFor));
		return { status: 200, body: { accepted: inputs.length } };
	}

	private async finish(handoff: WaitingHandoff, offered: unknown): Promise<Served> {
		const outcome: HandoffOutcome = offered === 'abandoned' ? 'abandoned' : 'completed';
		await this.end(handoff.handoffID, outcome);
		return { status: 200, body: { outcome } };
	}

	private async streamFor(handoff: WaitingHandoff, memberID: string, viewport: Viewport | null): Promise<HandoffStream> {
		if (handoff.stream?.isLive && handoff.stream.memberID === memberID) {
			if (viewport) await handoff.stream.resize(viewport);
			return handoff.stream;
		}
		if (handoff.opening) return handoff.opening;
		handoff.opening = this.openStream(handoff, memberID, viewport).finally(() => {
			handoff.opening = null;
		});
		return handoff.opening;
	}

	private async openStream(handoff: WaitingHandoff, memberID: string, viewport: Viewport | null): Promise<HandoffStream> {
		handoff.stream?.stop();
		const stream = new HandoffStream({
			handoffID: handoff.handoffID,
			memberID,
			page: await this.settings.openPage(handoff.devtoolsURL),
			deliver: this.settings.deliver,
			now: this.now,
			report: this.report
		});
		handoff.stream = stream;
		await stream.start(viewport ?? defaultViewport);
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
			const page = handoff.stream?.isLive ? handoff.stream.page : await this.settings.openPage(handoff.devtoolsURL);
			const where = await whereThePageIs(page);
			if (!handoff.stream?.isLive) page.close();
			return where;
		} catch (refusal) {
			this.report(`browser handoff ${handoff.handoffID} could not read the page: ${String(refusal)}`);
			return { url: '', title: '' };
		}
	}
}

type PageWhereabouts = { url: string; title: string };

async function whereThePageIs(page: DevtoolsConnection): Promise<PageWhereabouts> {
	const evaluated = await page.send('Runtime.evaluate', {
		expression: 'JSON.stringify({ url: location.href, title: document.title })',
		returnByValue: true
	});
	return whereaboutsOf(evaluated);
}

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
	viewport: Viewport = defaultViewport;
	private leaseRenewedAt = 0;
	private leaseCheck: ReturnType<typeof setInterval> | null = null;
	private lastImage = '';
	private url = '';
	private isStopped = false;
	private isCapturing = false;
	private fields: FieldBox[] = [];
	private isReadingFields = false;
	private fieldsReadAt = 0;
	private fieldReading: ReturnType<typeof setTimeout> | null = null;
	private commandsWaiting: DevtoolsCommand[] = [];
	private isDispatching = false;
	private troubleReportedAt = 0;

	constructor(private readonly settings: HandoffStreamSettings) {
		this.memberID = settings.memberID;
		this.page = settings.page;
	}

	get isLive(): boolean {
		return !this.isStopped && this.page.isOpen;
	}

	async start(viewport: Viewport): Promise<void> {
		this.page.onEvent((event) => this.onPageEvent(event));
		this.page.whenClosed(() => this.stop());
		await this.page.send('Page.enable');
		this.url = (await whereThePageIs(this.page)).url;
		await this.showAt(viewport);
		this.leaseCheck = setInterval(() => this.stopWhenNobodyWatches(), watchLeaseMilliseconds / 3);
	}

	async resize(viewport: Viewport): Promise<void> {
		if (viewport.width === this.viewport.width && viewport.height === this.viewport.height) return;
		await this.page.send('Page.stopScreencast');
		await this.showAt(viewport);
	}

	private async showAt(viewport: Viewport): Promise<void> {
		this.viewport = viewport;
		await this.page.send('Emulation.setDeviceMetricsOverride', { ...viewport, deviceScaleFactor: 1, mobile: false });
		await this.page.send('Page.startScreencast', { format: 'jpeg', quality: jpegQuality, maxWidth: viewport.width, maxHeight: viewport.height });
	}

	renewLease(): void {
		this.leaseRenewedAt = this.settings.now();
	}

	take(commands: DevtoolsCommand[]): void {
		this.commandsWaiting = commands.reduce(withWaitingCommand, this.commandsWaiting);
		void this.dispatchWhatWaits();
	}

	private async dispatchWhatWaits(): Promise<void> {
		if (this.isDispatching) return;
		this.isDispatching = true;
		while (this.isLive) {
			const command = this.commandsWaiting.shift();
			if (!command) break;
			await this.page.send(command.method, command.params).catch((refusal) => this.tellOfTrouble(command, refusal));
		}
		this.isDispatching = false;
		for (const delay of captureDelaysAfterInputMilliseconds) setTimeout(() => this.captureNow(), delay);
	}

	private tellOfTrouble(command: DevtoolsCommand, refusal: unknown): void {
		const reason = refusal instanceof Error ? refusal.message : String(refusal);
		this.settings.report(`browser handoff ${this.settings.handoffID} could not apply ${command.method}: ${reason}`);
		if (this.settings.now() - this.troubleReportedAt < troubleReportGapMilliseconds) return;
		this.troubleReportedAt = this.settings.now();
		this.settings.deliver({ kind: handoffTroubleEventKind, handoffID: this.settings.handoffID, reason }, this.memberID);
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
		if (this.fieldReading) clearTimeout(this.fieldReading);
		this.commandsWaiting = [];
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
		this.deliverFrame();
		this.refreshFields();
	}

	private deliverFrame(): void {
		this.settings.deliver(
			{ kind: handoffFrameEventKind, handoffID: this.settings.handoffID, image: this.lastImage, ...this.viewport, url: this.url, fields: this.fields },
			this.memberID
		);
	}

	private refreshFields(): void {
		if (this.isReadingFields || this.fieldReading || !this.isLive) return;
		const wait = this.fieldsReadAt + fieldReadingGapMilliseconds - this.settings.now();
		if (wait > 0) {
			this.fieldReading = setTimeout(() => {
				this.fieldReading = null;
				this.refreshFields();
			}, wait);
			return;
		}
		this.fieldsReadAt = this.settings.now();
		this.isReadingFields = true;
		editableFieldsOn(this.page)
			.then((fields) => {
				if (haveSameFields(fields, this.fields) || !this.isLive) return;
				this.fields = fields;
				this.deliverFrame();
			})
			.catch((refusal) => this.settings.report(`browser handoff ${this.settings.handoffID} could not find the fields: ${String(refusal)}`))
			.finally(() => {
				this.isReadingFields = false;
			});
	}
}

function withWaitingCommand(waiting: DevtoolsCommand[], command: DevtoolsCommand): DevtoolsCommand[] {
	const last = waiting.at(-1);
	if (last && isHover(last) && isHover(command)) return [...waiting.slice(0, -1), command];
	return [...waiting, command];
}

function isHover(command: DevtoolsCommand): boolean {
	return command.method === 'Input.dispatchMouseEvent' && command.params.type === 'mouseMoved' && command.params.button === 'none';
}

export type FieldBox = [x: number, y: number, width: number, height: number];

const typableInputTypes = ['text', 'search', 'email', 'url', 'tel', 'password', 'number'];
const largestFieldCount = 50;
const editableFieldsExpression = `JSON.stringify(
	[...document.querySelectorAll('input, textarea, [contenteditable]:not([contenteditable="false"])')]
		.filter((element) => !element.disabled && !element.readOnly)
		.filter((element) => element.tagName !== 'INPUT' || ${JSON.stringify(typableInputTypes)}.includes(element.type))
		.filter((element) => !element.checkVisibility || element.checkVisibility({ visibilityProperty: true }))
		.map((element) => element.getBoundingClientRect())
		.filter((box) => box.width > 0 && box.height > 0 && box.bottom > 0 && box.right > 0 && box.top < innerHeight && box.left < innerWidth)
		.slice(0, ${largestFieldCount})
		.map((box) => [box.left, box.top, box.width, box.height].map(Math.round))
)`;

async function editableFieldsOn(page: DevtoolsConnection): Promise<FieldBox[]> {
	const evaluated = await page.send('Runtime.evaluate', { expression: editableFieldsExpression, returnByValue: true });
	const value = recordOf(evaluated.result).value;
	if (typeof value !== 'string') return [];
	return fieldBoxesOf(JSON.parse(value));
}

function fieldBoxesOf(offered: unknown): FieldBox[] {
	if (!Array.isArray(offered)) return [];
	return offered.flatMap((box: unknown): FieldBox[] => {
		if (!Array.isArray(box) || box.length !== 4) return [];
		const [x, y, width, height]: unknown[] = box;
		if (typeof x !== 'number' || typeof y !== 'number' || typeof width !== 'number' || typeof height !== 'number') return [];
		return [[x, y, width, height]];
	});
}

function haveSameFields(first: FieldBox[], second: FieldBox[]): boolean {
	return JSON.stringify(first) === JSON.stringify(second);
}

export function readViewport(offered: unknown): Viewport | null {
	const held = recordOf(offered);
	if (typeof held.width !== 'number' || typeof held.height !== 'number') return null;
	if (!Number.isFinite(held.width) || !Number.isFinite(held.height)) return null;
	return { width: viewportSideOf(held.width), height: viewportSideOf(held.height) };
}

function viewportSideOf(offered: number): number {
	return Math.min(Math.max(Math.round(offered), smallestViewportSide), largestViewportSide);
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
