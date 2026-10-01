import { describe, expect, mock, test } from 'bun:test';

let localeValue = 'ko';

mock.module('../../../src/lib/i18n/locale.svelte', () => ({
	currentLocale: {
		get value() {
			return localeValue;
		}
	}
}));

const { createPageText } = await import('../../../src/lib/i18n/page-text.svelte');

const messages = {
	ko: {
		report: {
			title: '보고',
			weekdays: ['월', '화', '수']
		}
	},
	en: {
		report: {
			title: 'Report',
			weekdays: ['Mon', 'Tue', 'Wed']
		}
	}
} as const;

describe('createPageText', () => {
	test('returns localized array values for list copy', () => {
		const text = createPageText(messages);

		localeValue = 'en';
		expect(text.report.title).toBe('Report');
		expect(text.report.weekdays).toEqual(['Mon', 'Tue', 'Wed']);

		localeValue = 'ko';
		expect(text.report.title).toBe('보고');
		expect(text.report.weekdays).toEqual(['월', '화', '수']);
	});
});
