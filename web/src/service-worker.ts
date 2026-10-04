/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />

import { build, files } from '$service-worker';
import { homePath } from '$lib/home-path';
import { isShippedFile } from '$lib/offline-shell';
import { readArriving } from '$lib/notifications/arriving';
import { openedNotificationMessage } from '$lib/notifications/opened-notification';
import { keepPendingDestination } from '$lib/notifications/pending-destination';

const worker = self as unknown as ServiceWorkerGlobalScope;
const cacheName = 'internkim-assets';
const shipped = new Set([...build, ...files]);
const immutable = new Set(build.filter((path) => path.includes('/_app/immutable/')));

worker.addEventListener('install', (event) => {
	event.waitUntil(
		caches.open(cacheName).then(() => worker.skipWaiting())
	);
});

worker.addEventListener('activate', (event) => {
	event.waitUntil(keepRequestedAssets().then(() => worker.clients.claim()));
});

async function keepRequestedAssets(): Promise<void> {
	const cache = await caches.open(cacheName);
	const held = await cache.keys();
	await Promise.all(held.filter((request) => !isShippedFile(request, location.origin, shipped)).map((request) => cache.delete(request)));
}

async function previouslyCachedAsset(pathname: string): Promise<Response | undefined> {
	for (const name of await caches.keys()) {
		if (name === cacheName || !name.startsWith('internkim-')) continue;
		const previous = await caches.open(name);
		const held = await previous.match(pathname);
		if (held) return held;
	}
	return undefined;
}

async function rememberAsset(cache: Cache, pathname: string, response: Response): Promise<void> {
	try {
		await cache.put(pathname, response.clone());
	} catch (error) {
		console.warn('the static asset could not be cached', error);
	}
}

worker.addEventListener('fetch', (event) => {
	if (!isShippedFile(event.request, location.origin, shipped)) return;
	event.respondWith(shippedFile(event.request));
});

worker.addEventListener('push', (event) => {
	const arriving = readArriving(readPushedJSON(event.data));
	event.waitUntil(
		worker.registration.showNotification(arriving.title, {
			body: arriving.body,
			tag: arriving.tag,
			icon: arriving.icon || '/icon-192.png',
			data: { openPath: arriving.openPath }
		})
	);
});

function readPushedJSON(pushed: PushMessageData | null): unknown {
	if (!pushed) return null;
	try {
		return pushed.json();
	} catch {
		return null;
	}
}

worker.addEventListener('notificationclick', (event) => {
	event.notification.close();
	const openPath = (event.notification.data as { openPath?: string } | null)?.openPath ?? homePath;
	event.waitUntil(openTheApp(openPath));
});

async function openTheApp(openPath: string): Promise<void> {
	await keepPendingDestination(openPath);
	const open = await worker.clients.matchAll({ type: 'window', includeUncontrolled: true });
	const here = open.find((client) => new URL(client.url).origin === location.origin);
	if (here) {
		await here.focus();
		here.postMessage({ type: openedNotificationMessage, openPath });
		return;
	}
	await worker.clients.openWindow(homePath);
}

async function shippedFile(request: Request): Promise<Response> {
	const cache = await caches.open(cacheName);
	const pathname = new URL(request.url).pathname;
	const held = await cache.match(pathname);
	if (immutable.has(pathname)) {
		if (held) return held;
		const previous = await previouslyCachedAsset(pathname);
		if (previous) {
			await rememberAsset(cache, pathname, previous);
			return previous;
		}
	}

	let response: Response;
	try {
		response = await fetch(request);
	} catch (error) {
		if (held) return held;
		const previous = await previouslyCachedAsset(pathname);
		if (previous) return previous;
		throw error;
	}
	if (response.ok) await rememberAsset(cache, pathname, response);
	return response;
}
