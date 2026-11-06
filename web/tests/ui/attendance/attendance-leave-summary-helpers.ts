import type { Locator, Page } from '@playwright/test';

export type Locale = 'ko' | 'en';

export type LeaveSummaryColumn = {
	label: string;
	value: string;
	columnWidth: number;
	labelLeft: number;
	valueLeft: number;
	labelTextWidth: number;
	valueTextWidth: number;
};

const localeStorageKey = 'internkim.locale';

export const usedLabelOf: Record<Locale, string> = { ko: '사용', en: 'Used' };

export async function switchLocale(page: Page, locale: Locale): Promise<void> {
	await page.evaluate(
		([key, value]) => localStorage.setItem(key, value),
		[localeStorageKey, locale] as const
	);
	await page.reload();
}

export async function measureLeaveBalanceSummary(card: Locator): Promise<LeaveSummaryColumn[]> {
	return card.evaluate((element) => {
		const grid = element.querySelector('[data-testid="leave-balance-summary-columns"]');
		if (!(grid instanceof HTMLElement)) {
			throw new Error('The leave balance summary has no column grid');
		}
		const textBox = (node: HTMLElement) => {
			const range = document.createRange();
			range.selectNodeContents(node);
			return range.getBoundingClientRect();
		};
		return Array.from(grid.children).map((child) => {
			const column = child as HTMLElement;
			const [label, value] = Array.from(column.children) as HTMLElement[];
			if (!label || !value) {
				throw new Error('A leave balance summary column is missing its label or its value');
			}
			const labelBox = textBox(label);
			const valueBox = textBox(value);
			return {
				label: label.textContent?.trim() ?? '',
				value: value.textContent?.trim() ?? '',
				columnWidth: column.getBoundingClientRect().width,
				labelLeft: Math.round(labelBox.left),
				valueLeft: Math.round(valueBox.left),
				labelTextWidth: labelBox.width,
				valueTextWidth: valueBox.width
			};
		});
	});
}
