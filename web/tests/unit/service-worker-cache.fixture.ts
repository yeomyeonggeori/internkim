import { mock } from 'bun:test';

const origin = 'https://workspace.example.test';
const requestedAsset = '/_app/immutable/entry/app.current.js';
const nextAsset = '/_app/immutable/chunks/unused-0.js';
const shipped = [requestedAsset, ...Array.from({ length: 1000 }, (_, index) => `/_app/immutable/chunks/unused-${index}.js`)];
mock.module('$service-worker', () => ({ build: shipped, files: ['/icon-192.png'], version: 'current' }));

type WorkerEvent = {
	request: Request;
	waitUntil: (work: Promise<unknown>) => void;
	respondWith: (response: Promise<Response>) => void;
	data: { json: () => unknown };
	notification: { data: { openPath: string }; close: () => void };
};
const listeners = new Map<string, (event: WorkerEvent) => void>();
const stored = new Map<string, Map<string, Response>>([
	['internkim-previous', new Map([
		[requestedAsset, new Response('previously visited asset')],
		['/icon-192.png', new Response('previous icon')]
	])],
	['internkim-assets', new Map([['/_app/immutable/chunks/removed.old.js', new Response('removed')]])],
	['internkim-notification-destination', new Map()],
	['another-app-cache', new Map()]
]);
let skipWaiting = 0;
let claimed = 0;
let networkAvailable = true;
const fetched: string[] = [];
let eagerAssets = 0;
const notifications: { title: string; options: unknown }[] = [];
const messages: unknown[] = [];
let focused = 0;
let closed = 0;
let refuseCacheWrites = false;
let iconBody = 'current icon';

Reflect.set(globalThis, 'self', {
	addEventListener: (name: string, listener: (event: WorkerEvent) => void) => listeners.set(name, listener),
	skipWaiting: async () => { skipWaiting += 1; },
	clients: {
		claim: async () => { claimed += 1; },
		matchAll: async () => [{
			url: `${origin}/attendance`,
			focus: async () => { focused += 1; },
			postMessage: (message: unknown) => { messages.push(message); }
		}]
	},
	registration: {
		showNotification: async (title: string, options: unknown) => { notifications.push({ title, options }); }
	}
});
Reflect.set(globalThis, 'location', new URL(origin));
Reflect.set(globalThis, 'caches', {
	keys: async () => [...stored.keys()],
	delete: async (name: string) => stored.delete(name),
	open: async (name: string) => {
		let entries = stored.get(name);
		if (!entries) { entries = new Map(); stored.set(name, entries); }
		const cache = entries;
		const keyOf = (request: string | Request) => typeof request === 'string' ? request : new URL(request.url).pathname;
		return {
			addAll: async (paths: string[]) => { eagerAssets += paths.length; },
			keys: async () => [...cache.keys()].map(path => new Request(`${origin}${path}`)),
			match: async (request: string | Request) => cache.get(keyOf(request))?.clone(),
			put: async (path: string, response: Response) => {
				if (refuseCacheWrites) throw new Error('cache storage full');
				cache.set(path, response);
			},
			delete: async (request: string | Request) => cache.delete(keyOf(request))
		};
	}
});
Reflect.set(globalThis, 'fetch', async (request: Request) => {
	if (!networkAvailable) throw new Error('offline');
	fetched.push(new URL(request.url).pathname);
	return new Response(new URL(request.url).pathname === '/icon-192.png' ? iconBody : 'requested asset');
});

await import('../../src/service-worker');

async function dispatch(name: string, path = '/'): Promise<Response | undefined> {
	const work: Promise<unknown>[] = [];
	let response: Promise<Response> | undefined;
	listeners.get(name)?.({
		request: new Request(`${origin}${path}`),
		waitUntil: pending => { work.push(pending); },
		respondWith: pending => { response = pending; },
		data: { json: () => ({ title: 'Task changed', body: 'Review task', openPath: '/task?task=sample', tag: 'task-sample' }) },
		notification: { data: { openPath: '/task?task=sample' }, close: () => { closed += 1; } }
	});
	await Promise.all(work);
	return response;
}

await dispatch('install');
const afterInstall = { eagerAssets, networkRequests: fetched.length, skipWaiting };
refuseCacheWrites = true;
await dispatch('activate');
const afterActivate = { names: [...stored.keys()].sort(), assets: [...(stored.get('internkim-assets')?.keys() ?? [])].sort(), claimed };
networkAvailable = false;
const previous = await dispatch('fetch', requestedAsset);
const previousOfflineText = await previous?.text();
const previousIcon = await dispatch('fetch', '/icon-192.png');
const previousOfflineIcon = await previousIcon?.text();
refuseCacheWrites = false;
networkAvailable = true;
const currentIcon = await dispatch('fetch', '/icon-192.png');
const currentIconText = await currentIcon?.text();
refuseCacheWrites = true;
iconBody = 'newest icon';
const uncachedIcon = await dispatch('fetch', '/icon-192.png');
const uncachedFreshIcon = await uncachedIcon?.text();
refuseCacheWrites = false;
const first = await dispatch('fetch', nextAsset);
const onlineText = await first?.text();
networkAvailable = false;
const second = await dispatch('fetch', nextAsset);
const offlineText = await second?.text();
const privateResponse = await dispatch('fetch', '/api/member/me');
await dispatch('push');
await dispatch('notificationclick');
await dispatch('activate');
const pendingDestination = await stored.get('internkim-notification-destination')?.get('/internkim/notification-destination')?.clone().text();
console.log(JSON.stringify({
	afterInstall,
	afterActivate,
	previousOfflineText,
	previousOfflineIcon,
	currentIconText,
	uncachedFreshIcon,
	onlineText,
	offlineText,
	fetched,
	privateIntercepted: privateResponse !== undefined,
	notifications,
	messages,
	focused,
	closed,
	pendingDestination
}));
