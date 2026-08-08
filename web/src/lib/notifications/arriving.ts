export type Arriving = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
};

const somethingHappened: Arriving = {
	title: 'InternKim',
	body: '',
	openPath: '/flow/',
	tag: 'internkim'
};

export function readArriving(pushed: unknown): Arriving {
	if (typeof pushed !== 'object' || pushed === null) return somethingHappened;

	const held = pushed as Record<string, unknown>;
	return {
		title: text(held.title) || somethingHappened.title,
		body: text(held.body),
		openPath: ownPath(held.openPath) || somethingHappened.openPath,
		tag: text(held.tag) || somethingHappened.tag
	};
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered : '';
}

function ownPath(offered: unknown): string {
	if (typeof offered !== 'string') return '';
	if (!offered.startsWith('/') || offered.startsWith('//')) return '';
	return offered;
}
