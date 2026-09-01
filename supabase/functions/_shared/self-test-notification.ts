import type { Notification } from './push-to-member-devices.ts';

export type SelfTest = { title?: unknown; body?: unknown };

const longestLine = 200;
const fallbackTitle = 'internkim';

export function selfTestNotification(asked: SelfTest): Notification {
	return {
		title: line(asked.title) || fallbackTitle,
		body: line(asked.body),
		openPath: '/settings',
		tag: 'notification-self-test'
	};
}

function line(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim().slice(0, longestLine) : '';
}
