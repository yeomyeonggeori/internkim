import { describe, expect, test } from 'bun:test';
import {
	askOnceAndGuide,
	holdDesktopSocket,
	needsAccessibility,
	serveDesktopWork,
	type DesktopAnswer,
	type DesktopSocketWiring,
	type DesktopWork,
	type WorkSocket
} from '$lib/native-shell/desktop-control';

type FakeSocket = WorkSocket & {
	url: string;
	protocols: string[];
	sent: unknown[];
	isClosed: boolean;
	open(): void;
	hand(frame: unknown): void;
	drop(): void;
};

function fakeSocket(url = '', protocols: string[] = []): FakeSocket {
	const opened: (() => void)[] = [];
	const framed: ((data: unknown) => void)[] = [];
	const closed: (() => void)[] = [];
	const socket: FakeSocket = {
		url,
		protocols,
		sent: [],
		isClosed: false,
		send: (frame) => socket.sent.push(JSON.parse(frame)),
		close: () => {
			socket.isClosed = true;
		},
		onOpen: (listener) => opened.push(listener),
		onFrame: (listener) => framed.push(listener),
		onClose: (listener) => closed.push(listener),
		open: () => opened.forEach((listener) => listener()),
		hand: (frame) => framed.forEach((listener) => listener(JSON.stringify(frame))),
		drop: () => closed.forEach((listener) => listener())
	};
	return socket;
}

function settled(): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, 0));
}

function heldOpen<Value>(): { promise: Promise<Value>; resolve: (value: Value) => void } {
	let resolve: (value: Value) => void = () => {};
	const promise = new Promise<Value>((settle) => {
		resolve = settle;
	});
	return { promise, resolve };
}

const snapshotWork = { arguments: ['snapshot', '--app', 'Finder', '-i'] };

describe('a desktop answering work from the gateway', () => {
	test('says it has room once it is connected', () => {
		const socket = fakeSocket();
		serveDesktopWork(socket, async () => ({ output: '' }));
		socket.open();
		expect(socket.sent).toEqual([{ kind: 'ready', free: 1 }]);
	});

	test('answers work with what the named program printed and has room again afterwards', async () => {
		const socket = fakeSocket();
		const ran: DesktopWork[] = [];
		serveDesktopWork(socket, async (work) => {
			ran.push(work);
			return { output: '{"ok":true}' };
		});
		socket.hand({ kind: 'work', requestID: 'request-1', work: { ...snapshotWork, program: 'agent-browser', text: 'hello' } });
		await settled();
		expect(ran).toEqual([{ ...snapshotWork, program: 'agent-browser', text: 'hello' }]);
		expect(socket.sent).toEqual([
			{ kind: 'ready', free: 0 },
			{ kind: 'result', requestID: 'request-1', status: 200, body: { output: '{"ok":true}' } },
			{ kind: 'ready', free: 1 }
		]);
	});

	test('has no room while the work is still running', async () => {
		const socket = fakeSocket();
		const running = heldOpen<DesktopAnswer>();
		serveDesktopWork(socket, () => running.promise);
		socket.hand({ kind: 'work', requestID: 'request-1', work: snapshotWork });
		await settled();
		expect(socket.sent).toEqual([{ kind: 'ready', free: 0 }]);
		running.resolve({ output: 'done' });
		await settled();
		expect(socket.sent.at(-1)).toEqual({ kind: 'ready', free: 1 });
	});

	test('passes a refusal through as a refusal', async () => {
		const socket = fakeSocket();
		serveDesktopWork(socket, async () => ({ refusal: 'the person has not let internkim use this computer' }));
		socket.hand({ kind: 'work', requestID: 'request-2', work: snapshotWork });
		await settled();
		expect(socket.sent).toContainEqual({
			kind: 'result',
			requestID: 'request-2',
			status: 200,
			body: { refusal: 'the person has not let internkim use this computer' }
		});
	});

	test('refuses work it cannot read instead of leaving the gateway waiting', async () => {
		const socket = fakeSocket();
		serveDesktopWork(socket, async () => ({ output: 'never' }));
		socket.hand({ kind: 'work', requestID: 'request-3', work: { arguments: [1, 2] } });
		await settled();
		expect(socket.sent).toContainEqual({
			kind: 'result',
			requestID: 'request-3',
			status: 200,
			body: { refusal: 'the work named no arguments' }
		});
	});
});

describe('holding the desktop socket', () => {
	type Wired = { wiring: DesktopSocketWiring; dialled: FakeSocket[]; changed: (isAllowed: boolean) => void; redials: number[] };

	function wired(isAllowed: boolean): Wired {
		const wire: Wired = {
			dialled: [],
			redials: [],
			changed: () => {},
			wiring: {
				plugin: {
					state: async () => ({ isAllowed, accessibility: 'granted' }),
					run: async () => ({ output: '' }),
					onChanged: async (listener) => {
						wire.changed = listener;
						return () => {};
					}
				},
				owner: async () => ({ companyID: 'company-1', accessToken: 'token-1' }),
				gatewayURL: () => 'wss://gateway.example.com/',
				openSocket: (url, protocols) => {
					const socket = fakeSocket(url, protocols);
					wire.dialled.push(socket);
					return socket;
				},
				later: (callback, milliseconds) => {
					wire.redials.push(milliseconds);
					callback();
				}
			}
		};
		return wire;
	}

	test('dials nothing while the person has not allowed it', async () => {
		const wire = wired(false);
		holdDesktopSocket(wire.wiring);
		await settled();
		expect(wire.dialled).toEqual([]);
	});

	test('dials the company desktop endpoint with the member token when allowed', async () => {
		const wire = wired(true);
		holdDesktopSocket(wire.wiring);
		await settled();
		expect(wire.dialled.map((socket) => [socket.url, socket.protocols])).toEqual([
			['wss://gateway.example.com/company/company-1/desktop', ['internkim.bearer.token-1']]
		]);
	});

	test('hangs up when the switch is turned off and dials again when it is turned on', async () => {
		const wire = wired(true);
		holdDesktopSocket(wire.wiring);
		await settled();
		wire.changed(false);
		expect(wire.dialled[0]?.isClosed).toBe(true);
		wire.changed(true);
		await settled();
		expect(wire.dialled).toHaveLength(2);
	});

	test('redials with a growing wait after the socket drops', async () => {
		const wire = wired(true);
		holdDesktopSocket(wire.wiring);
		await settled();
		wire.dialled[0]?.drop();
		await settled();
		wire.dialled[1]?.drop();
		await settled();
		expect(wire.redials).toEqual([1_000, 2_000]);
		expect(wire.dialled).toHaveLength(3);
	});

	test('lets go of the socket when stopped', async () => {
		const wire = wired(true);
		const stop = holdDesktopSocket(wire.wiring);
		await settled();
		stop();
		wire.dialled[0]?.drop();
		await settled();
		expect(wire.dialled[0]?.isClosed).toBe(true);
		expect(wire.dialled).toHaveLength(1);
	});
});

describe('asking once whether internkim may use this computer', () => {
	type Asked = { languages: string[]; guides: number };

	function plugin(isAllowed: boolean, accessibility: string | null) {
		const asked: Asked = { languages: [], guides: 0 };
		return {
			asked,
			plugin: {
				askOnce: async (language: 'ko' | 'en') => {
					asked.languages.push(language);
					return { isAllowed, accessibility };
				},
				openPermissionGuide: async (): Promise<DesktopAnswer> => {
					asked.guides += 1;
					return { output: '' };
				}
			}
		};
	}

	test('asks in the person\'s language and opens the Accessibility settings a Mac still lacks', async () => {
		const { asked, plugin: stand } = plugin(true, 'denied');
		const state = await askOnceAndGuide(stand, 'ko', 'macos');
		expect(state.isAllowed).toBe(true);
		expect(asked).toEqual({ languages: ['ko'], guides: 1 });
	});

	test('opens nothing when the person declines', async () => {
		const { asked, plugin: stand } = plugin(false, null);
		await askOnceAndGuide(stand, 'en', 'macos');
		expect(asked.guides).toBe(0);
	});

	test('opens nothing when the Mac already granted it, or on a computer that has no such setting', async () => {
		const granted = plugin(true, 'granted');
		await askOnceAndGuide(granted.plugin, 'en', 'macos');
		const windows = plugin(true, null);
		await askOnceAndGuide(windows.plugin, 'en', 'windows');
		expect([granted.asked.guides, windows.asked.guides]).toEqual([0, 0]);
	});

	test('needs Accessibility only on a Mac that allowed it without granting it', () => {
		expect(needsAccessibility({ isAllowed: true, accessibility: 'denied' }, 'macos')).toBe(true);
		expect(needsAccessibility({ isAllowed: true, accessibility: null }, 'macos')).toBe(true);
		expect(needsAccessibility({ isAllowed: false, accessibility: 'denied' }, 'macos')).toBe(false);
		expect(needsAccessibility({ isAllowed: true, accessibility: 'denied' }, 'linux')).toBe(false);
	});
});
