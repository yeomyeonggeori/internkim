import { takePendingDestination } from './pending-destination';

export const openedNotificationMessage = 'notification-opened';

export type OpenedNotification = { type: typeof openedNotificationMessage; openPath: string };

export function pathOfOpenedNotification(sent: unknown): string {
	if (typeof sent !== 'object' || sent === null) return '';
	const held = sent as Record<string, unknown>;
	if (held.type !== openedNotificationMessage) return '';
	if (typeof held.openPath !== 'string') return '';
	if (!held.openPath.startsWith('/') || held.openPath.startsWith('//')) return '';
	return held.openPath;
}

export function goWhereNotificationsPoint(go: (path: string) => void): () => void {
	const follow = async () => {
		const path = await takePendingDestination();
		if (path) go(path);
	};
	const whenVisible = () => {
		if (document.visibilityState === 'visible') void follow();
	};

	void follow();
	document.addEventListener('visibilitychange', whenVisible);
	window.addEventListener('focus', whenVisible);

	if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) {
		return () => {
			document.removeEventListener('visibilitychange', whenVisible);
			window.removeEventListener('focus', whenVisible);
		};
	}

	const listen = (event: MessageEvent) => {
		const path = pathOfOpenedNotification(event.data);
		if (path) {
			void takePendingDestination();
			go(path);
		}
	};
	navigator.serviceWorker.addEventListener('message', listen);
	return () => {
		document.removeEventListener('visibilitychange', whenVisible);
		window.removeEventListener('focus', whenVisible);
		navigator.serviceWorker.removeEventListener('message', listen);
	};
}
