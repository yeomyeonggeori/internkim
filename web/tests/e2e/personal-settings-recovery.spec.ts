import { expect, test, type Page, type Route } from '@playwright/test';

const userDocument = {
	schemaVersion: 1,
	callMe: 'Sample',
	morningBriefing: { enabled: true, time: '11:00' }
};

async function mockSettingsPage(page: Page, userResponse: (route: Route, requestNumber: number) => Promise<void>, adminResponse?: (route: Route) => Promise<void>): Promise<void> {
	await page.route('**/auth/session**', (route) =>
		route.fulfill({ json: { authenticated: true, email: 'sample@example.com' } })
	);
	await page.route('**/admin/api/session', (route) =>
		adminResponse ? adminResponse(route) : route.fulfill({ json: { email: 'sample@example.com', isAdmin: false, role: 'member', isClaimed: true } })
	);
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'en' } }));
	await page.route('**/admin/api/workspace-settings', (route) =>
		route.fulfill({ json: { timeZone: 'Asia/Seoul', language: 'en' } })
	);

	let requestNumber = 0;
	await page.route('**/persona/api/user', (route) => {
		requestNumber += 1;
		return userResponse(route, requestNumber);
	});
	await page.route('**/persona/api/identity', (route) =>
		route.fulfill({ json: { schemaVersion: 1, names: ['Intern Kim'] } })
	);
	await page.route('**/persona/api/soul', (route) =>
		route.fulfill({ json: { schemaVersion: 1, values: [], boundaries: [], workingStyle: [] } })
	);
	await page.goto('/settings/');
}

function personalSettings(page: Page) {
	return page.getByRole('form', { name: 'How Intern Kim works with me' });
}

test('resolving the admin role preserves general settings and does not reload them', async ({ page }) => {
	let releaseRole = () => {};
	const roleReady = new Promise<void>((resolve) => { releaseRole = resolve; });
	let requests = 0;
	await mockSettingsPage(page, async (route, requestNumber) => {
		requests = requestNumber;
		await route.fulfill({ json: userDocument });
	}, async (route) => {
		await roleReady;
		await route.fulfill({ json: { email: 'sample@example.com', isAdmin: true, role: 'admin', isClaimed: true } });
	});
	const settings = personalSettings(page);
	await expect(settings.getByLabel('What to call me')).toHaveValue('Sample');
	await settings.getByLabel('What to call me').fill('Unsaved edit');
	releaseRole();
	await expect(page.getByRole('tab')).toHaveCount(2);
	await expect(settings.getByLabel('What to call me')).toHaveValue('Unsaved edit');
	expect(requests).toBe(1);
});

test('explicit retry recovers personal settings without showing defaults after failure', async ({ page }) => {
	await mockSettingsPage(page, async (route, requestNumber) => {
		if (requestNumber === 1) {
			await route.fulfill({ status: 503, body: 'unavailable' });
			return;
		}
		await route.fulfill({ json: userDocument });
	});

	const settings = personalSettings(page);
	await expect(settings.getByText('Could not load your settings.')).toBeVisible();
	await expect(settings.getByRole('button', { name: 'Try again' })).toBeVisible();
	await expect(settings.getByLabel('What to call me')).toHaveCount(0);
	await expect(settings.getByLabel('Time to receive it')).toHaveCount(0);

	await settings.getByRole('button', { name: 'Try again' }).click();
	await expect(settings.getByLabel('What to call me')).toHaveValue('Sample');
	await expect(settings.getByLabel('Time to receive it')).toHaveValue('11:00');
});

test('online recovery uses one request and focus after loading preserves edits', async ({ page }) => {
	let resolveRecovery: (() => void) | undefined;
	const recovery = new Promise<void>((resolve) => {
		resolveRecovery = resolve;
	});
	let requestNumber = 0;
	await mockSettingsPage(page, async (route, currentRequestNumber) => {
		requestNumber = currentRequestNumber;
		if (currentRequestNumber === 1) {
			await route.fulfill({ status: 503, body: 'unavailable' });
			return;
		}
		await recovery;
		await route.fulfill({ json: userDocument });
	});

	const settings = personalSettings(page);
	await expect(settings.getByText('Could not load your settings.')).toBeVisible();
	await Promise.all([
		page.evaluate(() => window.dispatchEvent(new Event('online'))),
		page.evaluate(() => window.dispatchEvent(new Event('online')))
	]);
	await expect.poll(() => requestNumber).toBe(2);
	resolveRecovery?.();
	await expect(settings.getByLabel('What to call me')).toHaveValue('Sample');

	await settings.getByLabel('What to call me').fill('Changed locally');
	await page.evaluate(() => window.dispatchEvent(new Event('focus')));
	await expect(settings.getByLabel('What to call me')).toHaveValue('Changed locally');
	await expect.poll(() => requestNumber).toBe(2);
});

test('focus recovery reloads settings after a failed initial load', async ({ page }) => {
	let requestNumber = 0;
	await mockSettingsPage(page, async (route, currentRequestNumber) => {
		requestNumber = currentRequestNumber;
		if (currentRequestNumber === 1) {
			await route.fulfill({ status: 503, body: 'unavailable' });
			return;
		}
		await route.fulfill({ json: userDocument });
	});

	const settings = personalSettings(page);
	await expect(settings.getByText('Could not load your settings.')).toBeVisible();
	await page.evaluate(() => window.dispatchEvent(new Event('focus')));
	await expect(settings.getByLabel('What to call me')).toHaveValue('Sample');
	await expect.poll(() => requestNumber).toBe(2);
});
