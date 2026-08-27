/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />

import { build, files, version } from '$service-worker';
import { homePath } from '$lib/home-path';
import { isShippedFile } from '$lib/offline-shell';
import { readArriving } from '$lib/notifications/arriving';
import { openedNotificationMessage } from '$lib/notifications/opened-notification';
import { keepPendingDestination } from '$lib/notifications/pending-destination';

const worker = self as unknown as ServiceWorkerGlobalScope;
const cacheName = `internkim-${version}`;
const shipped = new Set([...build, ...files]);

worker.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(cacheName)
			.then((cache) => cache.addAll([...shipped]))
			.then(() => worker.skipWaiting())
	);
});

worker.addEventListener('activate', (event) => {
	event.waitUntil(forgetOlderVersions().then(() => worker.clients.claim()));
});

async function forgetOlderVersions(): Promise<void> {
	const names = await caches.keys();
	await Promise.all(names.filter((name) => name !== cacheName).map((name) => caches.delete(name)));
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
	if (held) return held;

	const response = await fetch(request);
	if (response.ok) await cache.put(pathname, response.clone());
	return response;
}
