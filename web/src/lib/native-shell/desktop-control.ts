import { frameOf, type Frame } from '$lib/host-presence';
import type { Locale } from '$lib/i18n/locale';
import { gatewayURL, supabase } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';
import { tauriDesktopControl } from './desktop-control-tauri';
import { isInsideTauriDesktopShell, shellPlatform } from './shell';

export type DesktopWork = { program?: string; arguments: string[]; text?: string };
export type DesktopAnswer = { output: string; usage?: string } | { refusal: string };
export type DesktopControlState = { isAllowed: boolean; accessibility: string | null };
export type RunDesktopWork = (work: DesktopWork) => Promise<DesktopAnswer>;

export type DesktopControlPlugin = {
	state(): Promise<DesktopControlState>;
	allow(language: Locale): Promise<DesktopControlState>;
	askOnce(language: Locale): Promise<DesktopControlState>;
	disallow(): Promise<DesktopControlState>;
	run: RunDesktopWork;
	openPermissionGuide(): Promise<DesktopAnswer>;
	onChanged(listener: (isAllowed: boolean) => void): Promise<() => void>;
};

export type WorkSocket = {
	send(frame: string): void;
	close(): void;
	onOpen(listener: () => void): void;
	onFrame(listener: (data: unknown) => void): void;
	onClose(listener: () => void): void;
};

export type DesktopOwner = { companyID: string; accessToken: string };

export type DesktopSocketWiring = {
	plugin: Pick<DesktopControlPlugin, 'state' | 'run' | 'onChanged'>;
	owner(): Promise<DesktopOwner | null>;
	gatewayURL(): string;
	openSocket(url: string, protocols: string[]): WorkSocket;
	later(callback: () => void, milliseconds: number): void;
};

type HandedWork = { requestID: string; work: DesktopWork | null };

const tokenProtocol = 'internkim.bearer.';
const firstRedialMilliseconds = 1_000;
const longestRedialMilliseconds = 30_000;

export async function desktopControlShell(): Promise<DesktopControlPlugin | null> {
	return isInsideTauriDesktopShell() ? tauriDesktopControl() : null;
}

let asking: Promise<DesktopControlState | null> | undefined;

export function askToUseThisComputerOnce(language: Locale): Promise<DesktopControlState | null> {
	const current = (asking ??= askThisComputer(language).finally(() => {
		asking = undefined;
	}));
	return current;
}

async function askThisComputer(language: Locale): Promise<DesktopControlState | null> {
	const plugin = await desktopControlShell();
	return plugin ? askOnceAndGuide(plugin, language, shellPlatform()) : null;
}

export async function askOnceAndGuide(
	plugin: Pick<DesktopControlPlugin, 'askOnce' | 'openPermissionGuide'>,
	language: Locale,
	platform: string
): Promise<DesktopControlState> {
	const state = await plugin.askOnce(language);
	await guideToAccessibilityWhenNeeded(plugin, state, platform);
	return state;
}

export async function guideToAccessibilityWhenNeeded(
	plugin: Pick<DesktopControlPlugin, 'openPermissionGuide'>,
	state: DesktopControlState,
	platform: string
): Promise<void> {
	if (!needsAccessibility(state, platform)) return;
	const answer = await plugin.openPermissionGuide();
	if ('refusal' in answer) console.warn('the Accessibility settings did not open', answer.refusal);
}

export function needsAccessibility(state: DesktopControlState, platform: string): boolean {
	return platform === 'macos' && state.isAllowed && state.accessibility !== 'granted';
}

export function keepDesktopControlled(): () => void {
	let stop = () => {};
	let isStopped = false;
	desktopControlShell().then(
		(plugin) => {
			if (!plugin || isStopped) return;
			stop = holdDesktopSocket(liveWiring(plugin));
		},
		(failure: unknown) => console.warn('this computer is not offering itself to internkim', failure)
	);
	return () => {
		isStopped = true;
		stop();
	};
}

export function holdDesktopSocket(wiring: DesktopSocketWiring): () => void {
	const holder = new DesktopSocketHolder(wiring);
	holder.start();
	return () => holder.stop();
}

class DesktopSocketHolder {
	private socket: WorkSocket | undefined;
	private isAllowed = false;
	private isStopped = false;
	private redialsInARow = 0;
	private stopListening = () => {};

	constructor(private readonly wiring: DesktopSocketWiring) {}

	start(): void {
		this.wiring.plugin.onChanged((isAllowed) => this.follow(isAllowed)).then(
			(stopListening) => {
				if (this.isStopped) stopListening();
				else this.stopListening = stopListening;
			},
			(failure: unknown) => console.warn('the desktop-control switch is not being followed', failure)
		);
		this.wiring.plugin.state().then(
			(state) => this.follow(state.isAllowed),
			(failure: unknown) => console.warn('the desktop-control switch could not be read', failure)
		);
	}

	stop(): void {
		this.isStopped = true;
		this.stopListening();
		this.hangUp();
	}

	private follow(isAllowed: boolean): void {
		this.isAllowed = isAllowed;
		if (isAllowed) this.dialWhenWanted();
		else this.hangUp();
	}

	private isWanted(): boolean {
		return this.isAllowed && !this.isStopped && !this.socket;
	}

	private dialWhenWanted(): void {
		this.dial().catch((failure: unknown) => {
			console.warn('the desktop socket did not open', failure);
			this.redialLater();
		});
	}

	private async dial(): Promise<void> {
		if (!this.isWanted()) return;
		const owner = await this.wiring.owner();
		const address = this.wiring.gatewayURL();
		if (!owner || !address || !this.isWanted()) return;
		const socket = this.wiring.openSocket(desktopURL(address, owner.companyID), [tokenProtocol + owner.accessToken]);
		this.socket = socket;
		serveDesktopWork(socket, this.wiring.plugin.run);
		socket.onOpen(() => {
			this.redialsInARow = 0;
		});
		socket.onClose(() => this.dropped(socket));
	}

	private dropped(socket: WorkSocket): void {
		if (this.socket !== socket) return;
		this.socket = undefined;
		this.redialLater();
	}

	private redialLater(): void {
		if (!this.isWanted()) return;
		this.redialsInARow += 1;
		const delay = Math.min(firstRedialMilliseconds * 2 ** (this.redialsInARow - 1), longestRedialMilliseconds);
		this.wiring.later(() => this.dialWhenWanted(), delay);
	}

	private hangUp(): void {
		const socket = this.socket;
		this.socket = undefined;
		socket?.close();
	}
}

function desktopURL(address: string, companyID: string): string {
	return `${address.replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}/desktop`;
}

export function serveDesktopWork(socket: WorkSocket, run: RunDesktopWork): void {
	socket.onOpen(() => announceRoom(socket, 1));
	socket.onFrame((data) => {
		const handed = handedWorkOf(frameOf(data));
		if (handed) void answerHandedWork(socket, run, handed);
	});
}

async function answerHandedWork(socket: WorkSocket, run: RunDesktopWork, handed: HandedWork): Promise<void> {
	announceRoom(socket, 0);
	const body = handed.work ? await answerOf(run, handed.work) : { refusal: 'the work named no arguments' };
	socket.send(JSON.stringify({ kind: 'result', requestID: handed.requestID, status: 200, body }));
	announceRoom(socket, 1);
}

async function answerOf(run: RunDesktopWork, work: DesktopWork): Promise<DesktopAnswer> {
	try {
		return await run(work);
	} catch (failure) {
		const reason = failure instanceof Error ? failure.message : String(failure);
		return { refusal: `this computer could not run the work: ${reason}` };
	}
}

function announceRoom(socket: WorkSocket, free: number): void {
	socket.send(JSON.stringify({ kind: 'ready', free }));
}

function handedWorkOf(frame: Frame | null): HandedWork | null {
	if (frame?.kind !== 'work' || typeof frame.requestID !== 'string' || frame.requestID === '') return null;
	return { requestID: frame.requestID, work: desktopWorkOf(frame.work) };
}

function desktopWorkOf(value: unknown): DesktopWork | null {
	if (typeof value !== 'object' || value === null) return null;
	const record: Record<string, unknown> = { ...value };
	const { program, arguments: words, text } = record;
	if (!Array.isArray(words) || !words.every((word) => typeof word === 'string')) return null;
	return {
		arguments: words,
		...(typeof program === 'string' ? { program } : {}),
		...(typeof text === 'string' ? { text } : {})
	};
}

function liveWiring(plugin: DesktopControlPlugin): DesktopSocketWiring {
	return {
		plugin,
		owner: signedInDesktopOwner,
		gatewayURL,
		openSocket: (url, protocols) => workSocketOf(new WebSocket(url, protocols)),
		later: (callback, milliseconds) => void setTimeout(callback, milliseconds)
	};
}

async function signedInDesktopOwner(): Promise<DesktopOwner | null> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) return null;
	const { companyID } = await supabaseMember();
	return companyID ? { companyID, accessToken } : null;
}

function workSocketOf(socket: WebSocket): WorkSocket {
	return {
		send: (frame) => socket.send(frame),
		close: () => socket.close(),
		onOpen: (listener) => socket.addEventListener('open', listener),
		onFrame: (listener) => socket.addEventListener('message', (message) => listener(message.data)),
		onClose: (listener) => socket.addEventListener('close', listener)
	};
}
