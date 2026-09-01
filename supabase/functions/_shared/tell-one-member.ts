import { notificationCategories, type NotificationCategory } from './categories.ts';

export type Asked = {
	memberID?: unknown;
	category?: unknown;
	title?: unknown;
	body?: unknown;
	openPath?: unknown;
	tag?: unknown;
};

export type Telling = {
	memberID: string;
	category: NotificationCategory;
	title: string;
	body: string;
	openPath: string;
	tag: string;
};

const homePath = '/attendance/';
const longestLine = 200;

export function tellingAsked(asked: Asked, uniquely: string): Telling | null {
	const memberID = line(asked.memberID, 64);
	const category = asked.category;
	if (!memberID) return null;
	if (!isACategory(category)) return null;
	const title = line(asked.title, longestLine);
	if (!title) return null;
	return {
		memberID,
		category,
		title,
		body: line(asked.body, longestLine),
		openPath: line(asked.openPath, longestLine) || homePath,
		tag: line(asked.tag, longestLine) || `${category}-${uniquely}`
	};
}

function isACategory(offered: unknown): offered is NotificationCategory {
	return notificationCategories.some((category) => category === offered);
}

function line(offered: unknown, longest: number): string {
	return typeof offered === 'string' ? offered.trim().slice(0, longest) : '';
}
