export type DevtoolsEvent = {
	method: string;
	params: Record<string, unknown>;
};

type DevtoolsTarget = {
	type?: unknown;
	webSocketDebuggerUrl?: unknown;
};

type WaitingCommand = {
	resolve: (result: Record<string, unknown>) => void;
	reject: (reason: Error) => void;
};

const commandTimeoutMilliseconds = 15_000;

export class DevtoolsPage {
	private nextCommandID = 0;
	private readonly waiting = new Map<number, WaitingCommand>();
	private readonly listeners = new Set<(event: DevtoolsEvent) => void>();
	private readonly closeListeners = new Set<() => void>();

	private constructor(private readonly socket: WebSocket) {
		socket.addEventListener('message', (message) => this.receive(message.data));
		socket.addEventListener('close', () => this.forgetEverythingWaiting());
	}

	static async open(devtoolsURL: string): Promise<DevtoolsPage> {
		const socketURL = await pageSocketURLOf(devtoolsURL);
		const socket = new WebSocket(socketURL);
		await new Promise<void>((resolve, reject) => {
			socket.addEventListener('open', () => resolve(), { once: true });
			socket.addEventListener('error', () => reject(new Error(`the device browser at ${devtoolsURL} refused the page connection`)), {
				once: true
			});
		});
		return new DevtoolsPage(socket);
	}

	get isOpen(): boolean {
		return this.socket.readyState === WebSocket.OPEN;
	}

	send(method: string, params: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
		if (!this.isOpen) return Promise.reject(new Error(`the device browser page is closed, so ${method} was not sent`));
		const id = ++this.nextCommandID;
		const answered = new Promise<Record<string, unknown>>((resolve, reject) => {
			const timeout = setTimeout(() => {
				this.waiting.delete(id);
				reject(new Error(`the device browser did not answer ${method} in time`));
			}, commandTimeoutMilliseconds);
			this.waiting.set(id, {
				resolve: (result) => {
					clearTimeout(timeout);
					resolve(result);
				},
				reject: (reason) => {
					clearTimeout(timeout);
					reject(reason);
				}
			});
		});
		this.socket.send(JSON.stringify({ id, method, params }));
		return answered;
	}

	onEvent(listener: (event: DevtoolsEvent) => void): void {
		this.listeners.add(listener);
	}

	whenClosed(listener: () => void): void {
		this.closeListeners.add(listener);
	}

	close(): void {
		this.socket.close();
	}

	private forgetEverythingWaiting(): void {
		for (const waiting of this.waiting.values()) waiting.reject(new Error('the device browser page closed'));
		this.waiting.clear();
		for (const listener of this.closeListeners) listener();
	}

	private receive(data: unknown): void {
		const message = parsedMessageOf(data);
		if (!message) return;
		if (typeof message.id === 'number') {
			this.settle(message.id, message);
			return;
		}
		if (typeof message.method !== 'string') return;
		const event = { method: message.method, params: recordOf(message.params) };
		for (const listener of this.listeners) listener(event);
	}

	private settle(id: number, message: Record<string, unknown>): void {
		const waiting = this.waiting.get(id);
		if (!waiting) return;
		this.waiting.delete(id);
		const failure = recordOf(message.error);
		if (typeof failure.message === 'string') {
			waiting.reject(new Error(`the device browser refused: ${failure.message}`));
			return;
		}
		waiting.resolve(recordOf(message.result));
	}
}

async function pageSocketURLOf(devtoolsURL: string): Promise<string> {
	const response = await fetch(`${devtoolsURL.replace(/\/+$/, '')}/json/list`);
	if (!response.ok) throw new Error(`the device browser at ${devtoolsURL} answered ${response.status} for its page list`);
	const targets: unknown = await response.json();
	const page = Array.isArray(targets) ? targets.find(isPageTarget) : undefined;
	if (!page) throw new Error(`the device browser at ${devtoolsURL} has no open page`);
	return page.webSocketDebuggerUrl;
}

function isPageTarget(target: DevtoolsTarget): target is { type: 'page'; webSocketDebuggerUrl: string } {
	return target.type === 'page' && typeof target.webSocketDebuggerUrl === 'string';
}

function parsedMessageOf(data: unknown): Record<string, unknown> | null {
	if (typeof data !== 'string') return null;
	try {
		return recordOf(JSON.parse(data));
	} catch {
		return null;
	}
}

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}
