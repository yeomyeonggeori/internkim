import { homePath } from '$lib/home-path';
import { ownPath } from '$lib/return-path';

export type Arriving = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
	icon: string;
};

const somethingHappened: Arriving = {
	title: 'internkim',
	body: '',
	openPath: homePath,
	tag: 'internkim',
	icon: ''
};

export function readArriving(pushed: unknown): Arriving {
	if (typeof pushed !== 'object' || pushed === null) return somethingHappened;

	const held = pushed as Record<string, unknown>;
	return {
		title: text(held.title) || somethingHappened.title,
		body: text(held.body),
		openPath: ownPath(held.openPath) || somethingHappened.openPath,
		tag: text(held.tag) || somethingHappened.tag,
		icon: secureURL(held.icon)
	};
}

function secureURL(offered: unknown): string {
	if (typeof offered !== 'string') return '';
	return offered.startsWith('https://') ? offered : '';
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered : '';
}
