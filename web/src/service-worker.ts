/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />

import { build, files, version } from '$service-worker';
import { isShippedFile } from '$lib/offline-shell';

const worker = self as unknown as ServiceWorkerGlobalScope;
const cacheName = `internkim-${version}`;
const shipped = new Set([...build, ...files]);

worker.addEventListener('install', (event) => {
	event.waitUntil(caches.open(cacheName).then((cache) => cache.addAll([...shipped])));
});

worker.addEventListener('activate', (event) => {
	event.waitUntil(forgetOlderVersions());
});

async function forgetOlderVersions(): Promise<void> {
	const names = await caches.keys();
	await Promise.all(names.filter((name) => name !== cacheName).map((name) => caches.delete(name)));
}

worker.addEventListener('fetch', (event) => {
	if (!isShippedFile(event.request, location.origin, shipped)) return;
	event.respondWith(shippedFile(event.request));
});

async function shippedFile(request: Request): Promise<Response> {
	const cache = await caches.open(cacheName);
	const pathname = new URL(request.url).pathname;
	const held = await cache.match(pathname);
	if (held) return held;

	const response = await fetch(request);
	if (response.ok) await cache.put(pathname, response.clone());
	return response;
}
