import { Database } from 'bun:sqlite';
import { CompanyConnectionObject } from './company-connection';

type Closing = { code?: number; reason?: string };

export class TestSocket {
	readonly sent: string[] = [];
	readonly sentBinary: Uint8Array[] = [];
	closedWith: Closing | null = null;
	state: TestState | null = null;
	private attachment: unknown = null;

	send(document: string | Uint8Array): void {
		if (this.closedWith) throw new TypeError('the socket is closed');
		if (typeof document === 'string') this.sent.push(document);
		else this.sentBinary.push(document);
	}

	close(code?: number, reason?: string): void {
		if (this.closedWith) return;
		this.closedWith = { code, reason };
		this.state?.forget(this);
	}

	serializeAttachment(attachment: unknown): void {
		this.attachment = structuredClone(attachment);
	}

	deserializeAttachment(): unknown {
		return structuredClone(this.attachment);
	}

	receive(document: string | ArrayBuffer): void {
		this.state?.object?.webSocketMessage(this as unknown as WebSocket, document);
	}

	async drop(code = 1006, reason = ''): Promise<void> {
		const object = this.state?.object;
		this.state?.forget(this);
		await object?.webSocketClose(this as unknown as WebSocket, code, reason);
	}

	documents(): unknown[] {
		return this.sent.map((frame) => JSON.parse(frame));
	}
}

export let socketHeldByTheObject: TestSocket | null = null;

class TestWebSocketPair {
	readonly 0 = new TestSocket();
	readonly 1 = new TestSocket();

	constructor() {
		socketHeldByTheObject = this[1];
	}
}

class TestRequestResponsePair {
	constructor(
		readonly request: string,
		readonly response: string
	) {}
}

Object.assign(globalThis, { WebSocketPair: TestWebSocketPair, WebSocketRequestResponsePair: TestRequestResponsePair });

type SqlCursor = { toArray: () => Record<string, unknown>[] };

function sqlOver(database: Database): { exec: (query: string, ...bindings: (string | number | null)[]) => SqlCursor } {
	return {
		exec: (query, ...bindings) => {
			const rows = database.query(query).all(...bindings) as Record<string, unknown>[];
			return { toArray: () => rows };
		}
	};
}

export class TestState {
	object: CompanyConnectionObject | null = null;
	autoResponse: TestRequestResponsePair | null = null;
	readonly pongedAt = new Map<TestSocket, Date>();
	private readonly accepted = new Map<TestSocket, string[]>();
	readonly storage: unknown;

	constructor(
		readonly values: Map<string, unknown> = new Map(),
		readonly database: Database = new Database(':memory:')
	) {
		this.storage = {
			get: (key: string) => Promise.resolve(values.get(key)),
			put: (key: string, value: unknown) => {
				values.set(key, value);
				return Promise.resolve();
			},
			delete: (key: string) => Promise.resolve(values.delete(key)),
			sql: sqlOver(database)
		};
	}

	acceptWebSocket(socket: TestSocket, tags: string[] = []): void {
		socket.state = this;
		this.accepted.set(socket, tags);
	}

	getWebSockets(tag?: string): TestSocket[] {
		return [...this.accepted].filter(([, tags]) => !tag || tags.includes(tag)).map(([socket]) => socket);
	}

	setWebSocketAutoResponse(pair: TestRequestResponsePair): void {
		this.autoResponse = pair;
	}

	getWebSocketAutoResponseTimestamp(socket: TestSocket): Date | null {
		return this.pongedAt.get(socket) ?? null;
	}

	forget(socket: TestSocket): void {
		this.accepted.delete(socket);
	}

	wake(): TestState {
		const woken = new TestState(this.values, this.database);
		for (const [socket, tags] of this.accepted) woken.acceptWebSocket(socket, tags);
		return woken;
	}
}

export class ConnectionObject extends CompanyConnectionObject {
	constructor(state: TestState) {
		super(state as unknown as DurableObjectState);
		state.object = this;
	}
}

export function newState(values: Map<string, unknown> = new Map()): TestState {
	return new TestState(values);
}
