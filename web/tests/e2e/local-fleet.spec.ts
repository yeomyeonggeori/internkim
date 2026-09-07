import { expect, test } from '@playwright/test';
import { signInToTheTaskBoard } from './task-central-test-utils';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { member2Email } from './central-test-utils';

test.use({ locale: 'ko-KR' });

test('central plane serves an authenticated task board', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await expect(page.locator('[data-task-board-column="in_progress"]')).toBeVisible();
});

test('each member saves their own morning briefing preferences', async ({ page, browser, baseURL }) => {
	test.setTimeout(90000);
	await signInToTheCentralPlane(page, '/example-co/settings');
	const profile = page.getByRole('form', { name: '김인턴이 나를 대하는 방식' });
	await expect(profile.getByRole('button', { name: '저장', exact: true })).toBeEnabled();
	const enabled = profile.getByRole('switch', { name: '브리핑 받기' });
	const time = profile.getByLabel('받는 시간', { exact: true });
	await expect(time).toHaveValue('08:00');
	await expect(enabled).toBeChecked();
	await time.fill('');
	await profile.getByRole('button', { name: '저장', exact: true }).click();
	await expect.poll(() => time.evaluate((element) => element instanceof HTMLInputElement && element.validity.valueMissing)).toBe(true);
	await time.fill('09:25');
	await enabled.uncheck();
	await profile.getByRole('button', { name: '따뜻한', exact: true }).click();
	await profile.getByRole('button', { name: '저장', exact: true }).click();
	await expect(page.getByText('저장했습니다.', { exact: true })).toBeVisible();
	await page.reload();
	await expect(time).toHaveValue('09:25');
	await expect(enabled).not.toBeChecked();
	await expect(profile.getByRole('button', { name: '따뜻한', exact: true })).toHaveAttribute('aria-pressed', 'true');
	await profile.getByRole('button', { name: '저장', exact: true }).scrollIntoViewIfNeeded();
	await page.screenshot({ path: '.artifacts/personal-briefing/desktop.png' });
	const colleague = await browser.newPage({ baseURL, locale: 'ko-KR', viewport: { width: 390, height: 844 } });
	try {
		await signInToTheCentralPlane(colleague, '/example-co/settings', member2Email);
		const colleagueProfile = colleague.getByRole('form', { name: '김인턴이 나를 대하는 방식' });
		await expect(colleagueProfile.getByRole('button', { name: '저장', exact: true })).toBeEnabled();
		await expect(colleagueProfile.getByLabel('받는 시간', { exact: true })).toHaveValue('08:00');
		await expect(colleagueProfile.getByRole('switch', { name: '브리핑 받기' })).toBeChecked();
		await colleagueProfile.getByLabel('받는 시간', { exact: true }).fill('07:40');
		await colleagueProfile.getByRole('button', { name: '저장', exact: true }).click();
		await expect(colleague.getByText('저장했습니다.', { exact: true })).toBeVisible();
		await colleague.reload();
		await expect(colleagueProfile.getByLabel('받는 시간', { exact: true })).toHaveValue('07:40');
		await colleagueProfile.getByRole('button', { name: '저장', exact: true }).click({ trial: true });
		await colleague.screenshot({ path: '.artifacts/personal-briefing/mobile.png' });
		await colleagueProfile.getByLabel('받는 시간', { exact: true }).fill('08:00');
		await colleagueProfile.getByRole('button', { name: '저장', exact: true }).click();
		await expect(colleague.getByText('저장했습니다.', { exact: true })).toBeVisible();
	} finally {
		await colleague.close();
	}
	await time.fill('08:00');
	await enabled.check();
	await profile.getByRole('button', { name: '따뜻한', exact: true }).click();
	await profile.getByRole('button', { name: '저장', exact: true }).click();
	await expect(page.getByText('저장했습니다.', { exact: true })).toBeVisible();
});
