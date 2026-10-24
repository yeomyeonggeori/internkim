import type { DevtoolsCommand } from './browser-handoff-input';
import type { DevtoolsEvent } from './devtools-page';

export const handoffFrameEventKind = 'browser.handoff.frame';
export const handoffTroubleEventKind = 'browser.handoff.trouble';

export type DevtoolsConnection = {
	readonly isOpen: boolean;
	send: (method: string, params?: Record<string, unknown>) => Promise<Record<string, unknown>>;
	onEvent: (listener: (event: DevtoolsEvent) => void) => void;
	whenClosed: (listener: () => void) => void;
	close: () => void;
};

export type Viewport = { width: number; height: number };

export const defaultViewport: Viewport = { width: 1280, height: 800 };
const smallestViewportSide = 240;
const largestViewportSide = 2560;
const watchLeaseMilliseconds = 45_000;
const captureDelaysAfterInputMilliseconds = [80, 500];
const fieldReadingGapMilliseconds = 1_000;
const troubleReportGapMilliseconds = 5_000;
const jpegQuality = 60;

export type PageWhereabouts = { url: string; title: string };

export async function whereThePageIs(page: DevtoolsConnection): Promise<PageWhereabouts> {
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

export type HandoffStreamSettings = {
	handoffID: string;
	memberID: string;
	page: DevtoolsConnection;
	deliver: (event: Record<string, unknown>, memberID: string) => void;
	now: () => number;
	report: (line: string) => void;
};

export class HandoffStream {
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
		Promise.allSettled([this.page.send('Page.stopScreencast'), this.page.send('Emulation.clearDeviceMetricsOverride')]).finally(() =>
			this.page.close()
		);
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

function textOf(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}
