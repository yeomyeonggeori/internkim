const cacheName = 'internkim-notification-destination';
const address = '/internkim/notification-destination';

export function ownPath(offered: unknown): string {
	if (typeof offered !== 'string') return '';
	if (!offered.startsWith('/') || offered.startsWith('//')) return '';
	return offered;
}

export async function keepPendingDestination(openPath: string): Promise<void> {
	const path = ownPath(openPath);
	if (!path || typeof caches === 'undefined') return;
	const cache = await caches.open(cacheName);
	await cache.put(address, new Response(path));
}

export async function takePendingDestination(): Promise<string> {
	if (typeof caches === 'undefined') return '';
	const cache = await caches.open(cacheName).catch(() => null);
	if (!cache) return '';
	const held = await cache.match(address);
	if (!held) return '';
	await cache.delete(address);
	return ownPath(await held.text());
}
