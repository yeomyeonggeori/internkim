import { expect, test } from '@playwright/test';
import { exampleCompanySlug } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';

test.use({ locale: 'ko-KR' });

test('a reloaded attendance page shows its last view while the own record is still being read', async ({ page }) => {
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/attendance`);
	const ownStrip = page.getByTestId('attendance-own-strip');
	await expect(ownStrip).toBeVisible();
	let release = () => {};
	const gate = new Promise<void>((resolve) => { release = resolve; });
	await page.route(/\/api\/v1\/tools\/attendance_current_get\/invoke$/, async (route) => {
		await gate;
		await route.continue().catch(() => undefined);
	});
	try {
		await page.reload();
		await expect(ownStrip).toBeVisible();
		await expect(ownStrip.getByRole('button', { name: /출근|퇴근/ })).toBeDisabled();
		release();
		await expect(ownStrip.getByRole('button', { name: /출근|퇴근/ })).toBeEnabled();
	} finally {
		release();
	}
});

test('a failed read behind the last attendance view says so and offers a refresh', async ({ page }) => {
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/attendance`);
	const ownStrip = page.getByTestId('attendance-own-strip');
	await expect(ownStrip).toBeVisible();
	await page.route(/\/api\/v1\/tools\/attendance_current_get\/invoke$/, (route) =>
		route.fulfill({ status: 503, json: { error: 'Temporarily unavailable' } })
	);
	await page.reload();
	await expect(ownStrip).toBeVisible();
	await expect(page.getByRole('alert').filter({ hasText: 'Temporarily unavailable' })).toBeVisible();
	await page.unroute(/\/api\/v1\/tools\/attendance_current_get\/invoke$/);
	await page.getByRole('button', { name: '근태 새로고침', exact: true }).first().click();
	await expect(page.getByRole('alert').filter({ hasText: 'Temporarily unavailable' })).toHaveCount(0);
	await expect(ownStrip.getByRole('button', { name: /출근|퇴근/ })).toBeEnabled();
});
