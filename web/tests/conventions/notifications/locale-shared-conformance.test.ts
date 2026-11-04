import { describe, expect, test } from 'bun:test';
import { localeOf as browserLocaleOf } from '../../../src/lib/i18n/locale';
import { localeOf as sharedLocaleOf } from '../../../../supabase/functions/_shared/member-locale.ts';

describe('the language a notification is written in', () => {
	test('is read from a stored tag the way the app reads it', () => {
		for (const tag of ['ko', 'en', 'en-US', ' EN ', 'ko-KR', 'ja', '', null, undefined]) {
			expect(sharedLocaleOf(tag)).toBe(browserLocaleOf(tag));
		}
	});
});
