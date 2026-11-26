import { expect, test } from '@playwright/test';
import { signInToAttendance } from './attendance-central-test-utils';
import { switchLocale } from './attendance-leave-summary-helpers';

test.use({ locale: 'ko-KR' });

test('the time the team dashboard was read follows the language', async ({ page }) => {
	await signInToAttendance(page);
	await expect(page.getByText(/\d 기준$/)).toBeVisible();

	await switchLocale(page, 'en');

	await expect(page.getByText(/^As of .*\d/)).toBeVisible();
	await expect(page.getByText(/\d 기준$/)).toHaveCount(0);
});
