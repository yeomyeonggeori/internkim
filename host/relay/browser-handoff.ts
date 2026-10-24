import { mkdir, readFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import { devtoolsCommandsFor, readHandoffInputs } from './browser-handoff-input';
import { defaultViewport, HandoffStream, readViewport, type DevtoolsConnection, type Viewport } from './browser-handoff-stream';
import type { HandoffClaim } from './acp-session';
import { writeDurably } from './durable-write';

export const handoffsPath = '/browser-handoffs';
export const handoffWatchCapability = 'person.browser.handoff.watch';
export const handoffInputCapability = 'person.browser.handoff.input';
export const handoffFinishCapability = 'person.browser.handoff.finish';
export const handoffEndedEventKind = 'browser.handoff.ended';

const handoffCapabilities = new Set([handoffWatchCapability, handoffInputCapability, handoffFinishCapability]);

export function isBrowserHandoffCapability(capability: string): boolean {
	return handoffCapabilities.has(capability);
}

export type HandoffStatus = 'waiting' | 'open' | 'done' | 'abandoned' | 'expired';

export type HandoffRegistration = {
	handoffID: string;
	requesterEmail: string;
	devtoolsURL: string;
	signInOrigin: string;
	expiresAt: number;
	status: HandoffStatus;
};

export type Settle = () => void;

export type BrowserHandoffSettings = {
	appURL: string;
	filePath: string;
	openPage: (devtoolsURL: string) => Promise<DevtoolsConnection>;
	deliver: (event: Record<string, unknown>, memberID: string) => void;
	emailOfMember: (memberID: string) => Promise<string | null>;
	now?: () => number;
	unclaimedMilliseconds?: number;
	report?: (line: string) => void;
};

export type Served = { status: number; body: unknown };

type LiveHandoff = {
	registration: HandoffRegistration;
	expiry: ReturnType<typeof setTimeout> | null;
	settle: Settle | null;
	requesterMemberID: string | null;
	stream: HandoffStream | null;
	opening: Promise<HandoffStream> | null;
};

const defaultUnclaimedMilliseconds = 60_000;
const finishedStatuses = new Set<HandoffStatus>(['done', 'abandoned', 'expired']);

export class BrowserHandoffs {
	private readonly handoffs = new Map<string, LiveHandoff>();
	private readonly now: () => number;
	private readonly report: (line: string) => void;
	private writing: Promise<void> = Promise.resolve();

	constructor(private readonly settings: BrowserHandoffSettings) {
		this.now = settings.now ?? Date.now;
		this.report = settings.report ?? ((line) => console.log(line));
	}

	async restore(): Promise<void> {
		for (const registration of await this.readFromDisk()) this.keep(registration);
	}

	async register(offered: unknown): Promise<Served> {
		const registration = readRegistration(offered);
		if (!registration) return { status: 400, body: { error: 'a hand-over names its id, its requester, the browser and when it expires' } };
		if (this.handoffs.has(registration.handoffID)) return { status: 409, body: { error: 'this hand-over is already registered' } };
		this.keep(registration);
		this.armUnclaimed(registration.handoffID);
		await this.persist();
		return { status: 201, body: { handoffID: registration.handoffID } };
	}

	statusOf(handoffID: string): Served {
		const handoff = this.handoffs.get(handoffID);
		if (!handoff) return { status: 404, body: { error: 'no such hand-over' } };
		return { status: 200, body: { status: handoff.registration.status, signInOrigin: handoff.registration.signInOrigin } };
	}

	linkOf(handoffID: string): string {
		return `${this.settings.appURL.replace(/\/+$/, '')}/handoff/${encodeURIComponent(handoffID)}`;
	}

	async claim(handoffID: string, requesterEmail: string, settle: Settle): Promise<HandoffClaim> {
		const handoff = this.handoffs.get(handoffID);
		if (!handoff) return 'ended';
		if (handoff.registration.requesterEmail !== requesterEmail.trim().toLowerCase()) {
			this.report(`hand-over ${handoffID} was asked about in a session of someone other than its requester, so it was not opened`);
			return 'refused';
		}
		if (finishedStatuses.has(handoff.registration.status)) return 'ended';
		handoff.settle = settle;
		if (handoff.registration.status === 'waiting') {
			handoff.registration.status = 'open';
			await this.persist();
		}
		return 'open';
	}

	async answeredInTheThread(handoffID: string, isDone: boolean): Promise<void> {
		const handoff = this.handoffs.get(handoffID);
		if (!handoff) return;
		handoff.settle = null;
		await this.end(handoff, isDone ? 'done' : 'abandoned');
	}

	async serve(capability: string, body: Record<string, unknown>, memberID: string): Promise<Served> {
		const handoff = this.handoffs.get(textOf(body.handoffID));
		if (!handoff || handoff.registration.status !== 'open') return { status: 404, body: { error: 'this browser hand-over is no longer open' } };
		if (!(await this.isTheRequester(handoff, memberID))) return { status: 403, body: { error: 'this browser was handed to someone else' } };
		if (capability === handoffWatchCapability) return this.watch(handoff, memberID, body.viewport);
		if (capability === handoffInputCapability) return this.takeInputs(handoff, memberID, body.inputs);
		await this.end(handoff, body.outcome === 'abandoned' ? 'abandoned' : 'done');
		return { status: 200, body: { status: handoff.registration.status } };
	}

	close(): void {
		for (const handoff of this.handoffs.values()) {
			if (handoff.expiry) clearTimeout(handoff.expiry);
			handoff.stream?.stop();
		}
	}

	private keep(registration: HandoffRegistration): void {
		const handoff: LiveHandoff = { registration, expiry: null, settle: null, requesterMemberID: null, stream: null, opening: null };
		this.handoffs.set(registration.handoffID, handoff);
		if (finishedStatuses.has(registration.status)) return;
		handoff.expiry = setTimeout(() => void this.end(handoff, 'expired'), Math.max(0, registration.expiresAt - this.now()));
	}

	private armUnclaimed(handoffID: string): void {
		setTimeout(() => {
			const handoff = this.handoffs.get(handoffID);
			if (handoff?.registration.status === 'waiting') void this.end(handoff, 'expired');
		}, this.settings.unclaimedMilliseconds ?? defaultUnclaimedMilliseconds);
	}

	private async isTheRequester(handoff: LiveHandoff, memberID: string): Promise<boolean> {
		if (handoff.requesterMemberID === memberID) return true;
		const email = (await this.settings.emailOfMember(memberID))?.trim().toLowerCase();
		if (!email || email !== handoff.registration.requesterEmail) return false;
		handoff.requesterMemberID = memberID;
		return true;
	}

	private async watch(handoff: LiveHandoff, memberID: string, offeredViewport: unknown): Promise<Served> {
		const stream = await this.streamFor(handoff, memberID, readViewport(offeredViewport));
		stream.renewLease();
		stream.captureNow();
		return {
			status: 200,
			body: {
				handoffID: handoff.registration.handoffID,
				signInOrigin: handoff.registration.signInOrigin,
				expiresAt: new Date(handoff.registration.expiresAt).toISOString(),
				viewport: stream.viewport
			}
		};
	}

	private async takeInputs(handoff: LiveHandoff, memberID: string, offered: unknown): Promise<Served> {
		const inputs = readHandoffInputs(offered);
		if (!inputs) return { status: 400, body: { error: 'inputs must be a non-empty list of browser inputs' } };
		const stream = await this.streamFor(handoff, memberID, null);
		stream.renewLease();
		stream.take(inputs.flatMap(devtoolsCommandsFor));
		return { status: 200, body: { accepted: inputs.length } };
	}

	private async streamFor(handoff: LiveHandoff, memberID: string, viewport: Viewport | null): Promise<HandoffStream> {
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

	private async openStream(handoff: LiveHandoff, memberID: string, viewport: Viewport | null): Promise<HandoffStream> {
		handoff.stream?.stop();
		const stream = new HandoffStream({
			handoffID: handoff.registration.handoffID,
			memberID,
			page: await this.settings.openPage(handoff.registration.devtoolsURL),
			deliver: this.settings.deliver,
			now: this.now,
			report: this.report
		});
		handoff.stream = stream;
		await stream.start(viewport ?? defaultViewport);
		return stream;
	}

	private async end(handoff: LiveHandoff, status: HandoffStatus): Promise<void> {
		if (finishedStatuses.has(handoff.registration.status)) return;
		handoff.registration.status = status;
		if (handoff.expiry) clearTimeout(handoff.expiry);
		await this.persist();
		if (handoff.stream) {
			this.settings.deliver({ kind: handoffEndedEventKind, handoffID: handoff.registration.handoffID, status }, handoff.stream.memberID);
			handoff.stream.stop();
		}
		this.report(`hand-over ${handoff.registration.handoffID} ended ${status}`);
		const settle = handoff.settle;
		handoff.settle = null;
		settle?.();
	}

	private persist(): Promise<void> {
		const kept = [...this.handoffs.values()].map((handoff) => handoff.registration).filter((registration) => this.isWorthKeeping(registration));
		this.writing = this.writing.then(async () => {
			await mkdir(dirname(this.settings.filePath), { recursive: true });
			await writeDurably(this.settings.filePath, JSON.stringify(kept));
		});
		return this.writing;
	}

	private isWorthKeeping(registration: HandoffRegistration): boolean {
		return !finishedStatuses.has(registration.status) || registration.expiresAt > this.now();
	}

	private async readFromDisk(): Promise<HandoffRegistration[]> {
		const written = await readFile(this.settings.filePath, 'utf8').catch(() => null);
		if (written === null) return [];
		try {
			const offered: unknown = JSON.parse(written);
			return Array.isArray(offered) ? offered.flatMap((entry) => readKeptRegistration(entry) ?? []) : [];
		} catch {
			this.report(`the hand-overs at ${this.settings.filePath} will not parse and were left in place`);
			return [];
		}
	}
}

function readRegistration(offered: unknown): HandoffRegistration | null {
	const held = recordOf(offered);
	const handoffID = textOf(held.handoffID);
	const requesterEmail = textOf(held.requesterEmail).toLowerCase();
	const devtoolsURL = loopbackDevtoolsURLOf(held.devtoolsURL);
	const signInOrigin = textOf(held.signInOrigin);
	const expiresAt = Date.parse(textOf(held.expiresAt));
	if (!handoffID || !requesterEmail || !devtoolsURL || !signInOrigin || !Number.isFinite(expiresAt)) return null;
	return { handoffID, requesterEmail, devtoolsURL, signInOrigin, expiresAt, status: 'waiting' };
}

function readKeptRegistration(offered: unknown): HandoffRegistration | null {
	const held = recordOf(offered);
	const statuses: HandoffStatus[] = ['waiting', 'open', 'done', 'abandoned', 'expired'];
	const status = statuses.find((candidate) => candidate === held.status);
	const registration = readRegistration({ ...held, expiresAt: new Date(Number(held.expiresAt)).toISOString() });
	if (!registration || !status) return null;
	return { ...registration, status };
}

function loopbackDevtoolsURLOf(offered: unknown): string {
	const text = textOf(offered);
	if (!URL.canParse(text)) return '';
	const address = new URL(text);
	const isLoopback = address.hostname === '127.0.0.1' || address.hostname === 'localhost';
	if (address.protocol !== 'http:' || !isLoopback || !address.port) return '';
	return address.origin;
}

function textOf(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}
