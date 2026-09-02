import { describe, expect, test } from 'bun:test';
import {
	notificationCategories as webCategories,
	readNotificationSettings as webRead
} from '../../../src/lib/notifications/categories';
import {
	notificationCategories as sharedCategories,
	readNotificationSettings as sharedRead
} from '../../../../supabase/functions/_shared/categories.ts';

describe('the shared categories copy stays interchangeable with the web one', () => {
	test('both name the same categories in the same order', () => {
		expect([...sharedCategories]).toEqual([...webCategories]);
	});

	test('both read the same defaults and overrides', () => {
		expect(sharedRead(undefined)).toEqual(webRead(undefined));
		expect(sharedRead({ mail: true, task: false })).toEqual(webRead({ mail: true, task: false }));
	});
});
