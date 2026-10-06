import { expect, test, type Page } from '@playwright/test';
import { mkdir } from 'node:fs/promises';

const before = process.env.LOADING_EVIDENCE_PHASE === 'before';
test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

async function capture(page: Page, scene: string, width: number): Promise<void> {
	const folder = process.env.LOADING_EVIDENCE_DIR;
	if (!folder) return;
	await mkdir(folder, { recursive: true });
	await page.screenshot({ path: `${folder}/auth-${scene}-${width}-${before ? 'before' : 'after'}.png`, animations: 'disabled' });
}

for (const width of [1280, 390, 320]) {
	test(`authorization probe preserves an accessible loading status at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		await page.clock.setFixedTime(new Date('2026-10-06T03:00:00Z'));
		expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
		await page.route('**/auth/session**', route => route.fulfill({ json: { authenticated: false } }));
		await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
		const gate = Promise.withResolvers<void>();
		const requested = Promise.withResolvers<void>();
		await page.route('**/agent/api/buzz-relay-config', async route => {
			requested.resolve();
			await gate.promise;
			await route.fulfill({ json: { relayURL: 'wss://relay.example.test' } });
		});
		await page.goto('/task');
		await requested.promise;
		if (!before) await expect(page.getByTestId('web-auth-loading-status')).toBeVisible();
		await capture(page, 'checking-session', width);
		gate.resolve();
		await expect(page.getByLabel('이메일', { exact: true })).toBeVisible();
		if (!before) await expect(page.getByTestId('web-auth-loading-status')).toHaveCount(0);
		await capture(page, 'sign-in-ready', width);
	});
}
