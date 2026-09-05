import { expect, test } from '@playwright/test';
import { signInToTheTaskBoard } from './task-central-test-utils';

test.use({ locale: 'ko-KR' });

test('central plane serves an authenticated task board', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await expect(page.locator('[data-task-board-column="in_progress"]')).toBeVisible();
});
