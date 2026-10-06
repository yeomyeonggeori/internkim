import { expect, test, type Page } from '@playwright/test';
import { AttendanceLoadingFixture } from './attendance-loading-fixture';

async function returnToApp(page: Page): Promise<void> {
	await page.evaluate(() => { window.dispatchEvent(new Event('blur')); window.dispatchEvent(new Event('focus')); });
}

test.use({ locale: 'ko-KR' });

test('same authority revalidation retains content without remounting the page or shell', async ({ page }) => {
	const fixture = new AttendanceLoadingFixture();
	await fixture.install(page);
	await page.goto('/example-co/calendar?date=2026-10-06');
	await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
	const reads = fixture.calendar.reads;
	const membershipReads = fixture.membershipReads;
	await page.locator('.calendar-stage').evaluate(element => element.setAttribute('data-same-instance', 'true'));
	await returnToApp(page);
	await expect.poll(() => fixture.membershipReads).toBeGreaterThan(membershipReads);
	await expect(page.locator('.calendar-stage')).toHaveAttribute('data-same-instance', 'true');
	expect(fixture.calendar.reads).toBe(reads);
	await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
});

for (const changed of ['account', 'company', 'role']) {
	test(`${changed} change retires prior data and late reads while keeping the shell`, async ({ page }) => {
		const fixture = new AttendanceLoadingFixture();
		await fixture.install(page);
		await page.goto('/example-co/calendar?date=2026-10-06');
		await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
		const reads = fixture.calendar.reads;
		fixture.calendar.block();
		await page.getByRole('button', { name: '새로고침', exact: true }).first().click();
		await expect.poll(() => fixture.calendar.reads).toBeGreaterThan(reads);
		const previousRefreshReads = fixture.calendar.reads;
		fixture.calendarTitle = '새 범위의 일정';
		if (changed === 'company') fixture.identity.companyID = '20000000-0000-4000-8000-000000000002';
		if (changed === 'role') fixture.identity.isAdmin = false;
		if (changed === 'account') {
			fixture.identity.memberID = '10000000-0000-4000-8000-000000000002';
			fixture.identity.email = 'other@example.com';
			await page.evaluate(({ id, email }) => {
				const saved = JSON.parse(localStorage.getItem('sb-127-auth-token') || '{}');
				saved.user = { ...saved.user, id, email };
				const payload = { sub: id, iat: Math.floor(Date.now() / 1000), exp: Math.floor(Date.now() / 1000) + 36000 };
				const encoded = btoa(JSON.stringify(payload)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
				saved.access_token = `${saved.access_token.split('.')[0]}.${encoded}.fixture`;
				localStorage.setItem('sb-127-auth-token', JSON.stringify(saved));
			}, { id: fixture.identity.memberID, email: fixture.identity.email });
		}
		await page.locator('header').first().evaluate(element => element.setAttribute('data-same-shell', 'true'));
		await returnToApp(page);
		await expect.poll(() => fixture.calendar.reads).toBeGreaterThan(previousRefreshReads);
		await expect(page.getByText('제품 점검', { exact: true })).toHaveCount(0);
		await expect(page.locator('.calendar-stage')).toHaveAttribute('aria-busy', 'true');
		await expect(page.locator('header').first()).toHaveAttribute('data-same-shell', 'true');
		fixture.calendar.release();
		await expect(page.getByText('새 범위의 일정', { exact: true }).first()).toBeVisible();
		await expect(page.getByText('제품 점검', { exact: true })).toHaveCount(0);
	});
}

test('logout during a pending refresh cannot restore the previous workspace', async ({ page }) => {
	const fixture = new AttendanceLoadingFixture();
	await fixture.install(page);
	await page.goto('/example-co/calendar?date=2026-10-06');
	await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
	fixture.calendar.block();
	const reads = fixture.calendar.reads;
	await page.getByRole('button', { name: '새로고침', exact: true }).first().click();
	await expect.poll(() => fixture.calendar.reads).toBeGreaterThan(reads);
	await page.evaluate(() => localStorage.removeItem('sb-127-auth-token'));
	await returnToApp(page);
	await expect(page.locator('.calendar-stage')).toHaveCount(0);
	fixture.calendar.release();
	await expect(page.getByText('제품 점검', { exact: true })).toHaveCount(0);
	await expect(page.getByLabel('이메일', { exact: true })).toBeVisible();
});
