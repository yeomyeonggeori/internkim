import { expect, test, type Page } from '@playwright/test';

type StoredPersona = { document: Record<string, unknown>; saves: number };

test.use({ locale: 'ko-KR' });

async function openSettingsKeeping(page: Page, stored: StoredPersona): Promise<void> {
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email: 'sample@example.com' } }));
	await page.route('**/admin/api/session', (route) =>
		route.fulfill({ json: { email: 'sample@example.com', isAdmin: false, role: 'member', isClaimed: true } })
	);
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/admin/api/workspace-settings', (route) => route.fulfill({ json: { timeZone: 'Asia/Seoul', language: 'ko' } }));
	await page.route('**/persona/api/identity', (route) => route.fulfill({ json: { schemaVersion: 1, names: ['김인턴'] } }));
	await page.route('**/persona/api/user', async (route) => {
		if (route.request().method() === 'POST') {
			stored.saves += 1;
			stored.document = route.request().postDataJSON();
		}
		await route.fulfill({ json: stored.document });
	});
	await page.goto('/settings/');
}

test('a member moves the morning briefing, turns it off, and finds it that way after a reload', async ({ page }) => {
	const stored: StoredPersona = { document: { schemaVersion: 1, callMe: '이샘플', morningBriefing: { enabled: true, time: '08:00' } }, saves: 0 };
	await openSettingsKeeping(page, stored);
	const profile = page.getByRole('form', { name: '김인턴이 나를 대하는 방식' });
	const enabled = profile.getByRole('switch', { name: '브리핑 받기' });
	const time = profile.getByLabel('받는 시간', { exact: true });
	const save = profile.getByRole('button', { name: '저장', exact: true });
	await expect(time).toHaveValue('08:00');
	await expect(enabled).toBeChecked();

	await time.fill('');
	await save.click();
	await expect.poll(() => time.evaluate((element) => element instanceof HTMLInputElement && element.validity.valueMissing)).toBe(true);
	expect(stored.saves).toBe(0);

	await time.fill('09:25');
	await enabled.uncheck();
	await profile.getByRole('button', { name: '따뜻한', exact: true }).click();
	await save.click();
	await expect(page.getByText('저장했습니다.', { exact: true })).toBeVisible();
	expect(stored.document.morningBriefing).toEqual({ enabled: false, time: '09:25' });

	await page.reload();
	await expect(time).toHaveValue('09:25');
	await expect(enabled).not.toBeChecked();
	await expect(profile.getByRole('button', { name: '따뜻한', exact: true })).toHaveAttribute('aria-pressed', 'true');
});
