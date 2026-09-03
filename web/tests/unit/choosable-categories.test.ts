import { describe, expect, test } from 'bun:test';
import { notificationCategories } from '../../src/lib/notifications/categories';
import { categoriesChoosableBy } from '../../src/lib/server/public-api/record/notifications';

describe('categoriesChoosableBy', () => {
	test('an administrator chooses every category', () => {
		expect(categoriesChoosableBy(true)).toEqual([...notificationCategories]);
	});

	test('a member is not offered the leave requests others send', () => {
		expect(categoriesChoosableBy(false)).not.toContain('leave');
	});

	test('a member keeps every other category, in the same order', () => {
		const kept = notificationCategories.filter((category) => category !== 'leave');
		expect(categoriesChoosableBy(false)).toEqual(kept);
	});
});
