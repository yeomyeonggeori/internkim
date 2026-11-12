import { readdirSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { everyScreen } from '../support/screen-widths';

const uiComponentDirectories = readdirSync(new URL('../../../src/lib/components/ui', import.meta.url), { withFileTypes: true })
	.filter((entry) => entry.isDirectory())
	.map((entry) => entry.name);

async function openDesign(page: Page, frame = ''): Promise<void> {
	await page.addInitScript(() => localStorage.setItem('internkim.locale', 'ko'));
	await page.goto(`/design${frame ? `#${frame}` : ''}`);
	await expect(page.getByRole('heading', { name: '김인턴 디자인 시스템', level: 1 })).toBeVisible();
	await expect(page.getByText(/[1-9]\d* 프레임/)).toBeAttached({ timeout: 30_000 });
}

for (const width of everyScreen) {
	test(`the design workspace draws the messenger states without widening the page at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		await openDesign(page, 'messenger-agent-typing');

		await expect(page.getByRole('status').filter({ hasText: '김인턴이 입력 중이에요' }).first()).toBeVisible();
		await expect(page.locator('[data-frame="messenger-person-typing"]').getByRole('status', { name: '박예시님이 입력 중이에요' }).first()).toBeAttached();
		await expect(page.locator('[data-frame="messenger-agent-error"]').getByText('캘린더에 연결하지 못해 일정을 옮기지 못했어요.').first()).toBeAttached();
		await expect(page.locator('[data-frame="messenger-agent-choice"] [data-slot="choice-card"] [data-look="pressable"]').first()).toBeAttached();
		await expect(page.locator('[data-frame="messenger-agent-approval"]').getByRole('button', { name: '승인' }).first()).toBeAttached();
		await expect(page.locator('[data-frame="messenger-agent-choice-answered"] [data-look="chosen"]').first()).toBeAttached();
		await expect(page.locator('[data-frame="messenger-agent-presence"] [data-presence="online"]').first()).toBeAttached();

		const pageWidth = await page.evaluate(() => document.documentElement.scrollWidth);
		expect(pageWidth).toBeLessThanOrEqual(width);
	});
}

test('every ui component has a frame on the design page', async ({ page }) => {
	await openDesign(page);

	const drawnSources = await page.locator('[data-frame-source]').evaluateAll((frames) =>
		frames.map((frame) => frame.getAttribute('data-frame-source') ?? '')
	);
	const missing = uiComponentDirectories.filter((directory) => !drawnSources.includes(`ui/${directory}`));
	expect(missing).toEqual([]);
});

test('each frame draws its light and dark theme side by side, and one theme on request', async ({ page }) => {
	await page.setViewportSize({ width: 1600, height: 900 });
	await openDesign(page, 'button');

	const frame = page.locator('[data-frame="button"]');
	const backgrounds = await frame.locator('[data-theme-pane]').evaluateAll((panes) =>
		panes.map((pane) => getComputedStyle(pane).backgroundColor)
	);
	expect(backgrounds).toHaveLength(2);
	expect(backgrounds[0]).not.toBe(backgrounds[1]);

	await page.getByRole('radio', { name: '다크', exact: true }).click();
	await expect(frame.locator('[data-theme-pane]')).toHaveCount(1);
	await expect(frame.locator('[data-theme-pane="dark"]')).toBeVisible();
});

test('selecting a frame shows its source, variants and tokens', async ({ page }) => {
	await page.setViewportSize({ width: 1600, height: 900 });
	await openDesign(page, 'button');

	const inspector = page.getByRole('complementary', { name: '속성' });
	await expect(inspector.getByRole('heading', { name: 'Button', level: 2 })).toBeVisible();
	await expect(inspector.getByText('web/src/lib/components/ui/button')).toBeVisible();
	await expect(inspector.getByText('destructive', { exact: true }).first()).toBeVisible();
	await expect(inspector.getByText('primary-foreground', { exact: true })).toBeVisible();
});

test('the design page stays outside any company address', async ({ page }) => {
	await page.goto('/design');

	await expect(page).toHaveURL(/\/design$/);
	await expect(
		page.getByRole('heading', { name: 'internkim design system', level: 1 }).or(page.getByRole('heading', { name: '김인턴 디자인 시스템', level: 1 }))
	).toBeVisible();
});
