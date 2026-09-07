import { expect, test } from '@playwright/test';
import { signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

test('shows each KPI hero value in full instead of cutting it off', async ({ page }) => {
	await signInToTheCRM(page);

	const clipped = await page.locator('[data-crm-kpi-value]').evaluateAll((values) =>
		values
			.filter((value) => (value as HTMLElement).offsetParent !== null)
			.filter((value) => ((value.firstElementChild ?? value) as HTMLElement).scrollWidth > value.clientWidth)
			.map((value) => value.textContent?.trim() ?? '')
	);
	expect(clipped).toEqual([]);
});
