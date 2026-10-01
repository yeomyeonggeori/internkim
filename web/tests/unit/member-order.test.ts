import { describe, expect, test } from 'bun:test';
import { membersInReadingOrder } from '../../src/lib/member-order';

function member(id: string, name: string, joined_at: string | null) {
	return { id, name, email: `${id}@example.com`, joined_at };
}

describe('membersInReadingOrder', () => {
	test('the reader comes first, however late they joined', () => {
		const ordered = membersInReadingOrder(
			[member('a', '김예시', '2024-01-01'), member('b', '이샘플', '2026-06-01')],
			'b'
		);
		expect(ordered.map((entry) => entry.id)).toEqual(['b', 'a']);
	});

	test('everyone else reads by who joined earliest', () => {
		const ordered = membersInReadingOrder(
			[member('a', '박예시', '2026-03-01'), member('b', '김예시', '2024-01-01'), member('c', '곽샘플', '2025-05-05')],
			undefined
		);
		expect(ordered.map((entry) => entry.id)).toEqual(['b', 'c', 'a']);
	});

	test('a shared joining day falls back to the name', () => {
		const ordered = membersInReadingOrder(
			[member('a', '장샘플', '2026-01-01'), member('b', '곽샘플', '2026-01-01'), member('c', '김테스트', '2026-01-01')],
			undefined
		);
		expect(ordered.map((entry) => entry.name)).toEqual(['곽샘플', '김테스트', '장샘플']);
	});

	test('nobody claims the earliest place by having no joining day', () => {
		const ordered = membersInReadingOrder(
			[member('a', '가나다', null), member('b', '하하하', '2026-06-01')],
			undefined
		);
		expect(ordered.map((entry) => entry.id)).toEqual(['b', 'a']);
	});

	test('the time of day a record carries does not change the order', () => {
		const ordered = membersInReadingOrder(
			[
				{ id: 'a', name: '나중', email: 'a@example.com', joined_at: '2026-01-01T23:00:00Z' },
				{ id: 'b', name: '가장먼저', email: 'b@example.com', joined_at: '2026-01-01T01:00:00Z' }
			],
			undefined
		);
		expect(ordered.map((entry) => entry.name)).toEqual(['가장먼저', '나중']);
	});

	test('it leaves the list it was given alone', () => {
		const original = [member('a', '박예시', '2026-03-01'), member('b', '김예시', '2024-01-01')];
		membersInReadingOrder(original, undefined);
		expect(original.map((entry) => entry.id)).toEqual(['a', 'b']);
	});
});
