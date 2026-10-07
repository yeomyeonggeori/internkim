import { expect, test, type Page } from '@playwright/test';
import { prepareWorkspaceLoading, WorkspaceLoadingFixture } from './workspace-loading-fixture';

class CurrencyFixture extends WorkspaceLoadingFixture {
	baseCurrency = 'KRW';
	readonly rates: Record<string, number> = { 'USD:KRW': 1000, 'KRW:USD': 0.001, 'KRW:EUR': 0.0008, 'USD:EUR': 0.8 };

	override tool(name: string, input: Record<string, unknown>): unknown {
		if (name === 'company_settings_get') return { currencyCode: this.baseCurrency, timeZone: 'Asia/Seoul', name: '예시 회사' };
		return super.tool(name, input);
	}
}

async function openDeals(page: Page, fixture: CurrencyFixture) {
	await prepareWorkspaceLoading(page, fixture);
	await page.route('**/api/currencies', route => route.fulfill({ json: { currencies: [
		{ code: 'KRW', name: 'Korean Won', minorUnitDigits: 0 },
		{ code: 'USD', name: 'US Dollar', minorUnitDigits: 2 },
		{ code: 'EUR', name: 'Euro', minorUnitDigits: 2 }
	] } }));
	await page.route('**/api/currencies/conversion?**', async route => {
		const parameters = new URL(route.request().url()).searchParams;
		const from = parameters.get('from') ?? '';
		const to = parameters.get('to') ?? '';
		const pair = `${from}:${to}`;
		await fixture.waitFor(pair);
		if (fixture.refused.has(pair)) {
			await route.fulfill({ status: 502, json: { error: 'Fixture rate provider unavailable' } });
			return;
		}
		const rate = fixture.rates[pair];
		if (rate === undefined) throw new Error(`Unexpected conversion request: ${pair}`);
		await route.fulfill({ json: { amountMinor: 1000000 * rate, currencyCode: to, rate, asOf: '2026-10-06' } });
	});
	await page.goto('/example-co/crm');
	await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
	await page.getByRole('tab', { name: '거래', exact: true }).click();
	const panel = page.getByRole('tabpanel', { name: '거래', exact: true });
	const row = panel.getByRole('row').filter({ hasText: '협력 제안 1' });
	await expect(row).toBeVisible();
	const amount = row.locator('span.tabular-nums:visible, td.tabular-nums:visible').filter({ hasText: /^[₩$€]/ });
	return { panel, currency: panel.getByLabel('보기 통화', { exact: true }), amount };
}

async function choose(page: Page, code: string) {
	await page.getByRole('tabpanel', { name: '거래', exact: true }).getByLabel('보기 통화', { exact: true }).click();
	await page.getByRole('option', { name: new RegExp(`^${code} `) }).click();
}

test.use({ locale: 'ko-KR', colorScheme: 'light' });

test('cold pending base currency never labels unconverted amounts as the requested currency', async ({ page }) => {
	const fixture = new CurrencyFixture();
	fixture.baseCurrency = 'USD';
	fixture.hold('KRW:USD');
	const { panel, currency, amount } = await openDeals(page, fixture);
	await expect(currency).toHaveText('원래 통화');
	await expect(currency).toHaveAttribute('aria-busy', 'true');
	await expect(panel.getByRole('status')).toHaveText('USD 환율 불러오는 중…');
	await expect(amount).toHaveText('₩150만');
	fixture.release('KRW:USD');
	await expect(currency).toHaveText('USD');
	await expect(amount).toHaveText('$1.5천');
	await expect(currency).toHaveAttribute('aria-busy', 'false');
});

test('failed choice preserves amounts and selected option, with an explicit successful retry', async ({ page }) => {
	const fixture = new CurrencyFixture();
	const { panel, currency, amount } = await openDeals(page, fixture);
	await expect(currency).toHaveText('KRW');
	fixture.hold('KRW:USD');
	fixture.refused.add('KRW:USD');
	await choose(page, 'USD');
	await expect(currency).toHaveText('KRW');
	await expect(amount).toHaveText('₩150만');
	await expect(panel.getByRole('status')).toHaveText('USD 환율 불러오는 중…');
	fixture.release('KRW:USD');
	await expect(panel.getByRole('button', { name: '다시 시도', exact: true })).toBeVisible();
	await expect(panel.getByRole('status')).toContainText('환율을 불러오지 못했습니다.');
	await expect(currency).toHaveText('KRW');
	await expect(amount).toHaveText('₩150만');
	await currency.click();
	await expect(page.getByRole('option', { name: /^KRW / })).toHaveAttribute('aria-selected', 'true');
	await expect(page.getByRole('option', { name: /^USD / })).not.toHaveAttribute('aria-selected', 'true');
	await page.keyboard.press('Escape');
	fixture.refused.delete('KRW:USD');
	await panel.getByRole('button', { name: '다시 시도', exact: true }).click();
	await expect(currency).toHaveText('USD');
	await expect(amount).toHaveText('$1.5천');
	await expect(panel.locator('[data-crm-view-currency]').getByRole('status')).toHaveCount(0);
});

test('the latest currency choice wins when an older response arrives afterward', async ({ page }) => {
	const fixture = new CurrencyFixture();
	const { currency, amount } = await openDeals(page, fixture);
	await expect(currency).toHaveText('KRW');
	fixture.hold('KRW:USD');
	await choose(page, 'USD');
	await expect(currency).toHaveAttribute('aria-busy', 'true');
	await choose(page, 'EUR');
	await expect(currency).toHaveText('EUR');
	await expect(amount).toHaveText('€1.2천');
	const olderResponse = page.waitForResponse(response => response.url().includes('/api/currencies/conversion?') && new URL(response.url()).searchParams.get('to') === 'USD');
	fixture.release('KRW:USD');
	await olderResponse;
	await expect(currency).toHaveText('EUR');
	await expect(amount).toHaveText('€1.2천');
	await expect(currency).toHaveAttribute('aria-busy', 'false');
});

test('a stalled conversion request reaches a retryable error within the client deadline', async ({ page }) => {
	const fixture = new CurrencyFixture();
	const { panel, currency, amount } = await openDeals(page, fixture);
	await expect(currency).toHaveText('KRW');
	fixture.hold('KRW:USD');
	await choose(page, 'USD');
	await expect(panel.getByRole('status')).toHaveText('USD 환율 불러오는 중…');
	await expect(panel.getByRole('button', { name: '다시 시도', exact: true })).toBeVisible({ timeout: 10000 });
	await expect(currency).toHaveAttribute('aria-busy', 'false');
	await expect(currency).toHaveText('KRW');
	await expect(amount).toHaveText('₩150만');
	fixture.release('KRW:USD');
	await panel.getByRole('button', { name: '다시 시도', exact: true }).click();
	await expect(currency).toHaveText('USD');
	await expect(amount).toHaveText('$1.5천');
});

test('cancelling a pending choice keeps the previous conversion after its late response', async ({ page }) => {
	const fixture = new CurrencyFixture();
	const { panel, currency, amount } = await openDeals(page, fixture);
	await expect(currency).toHaveText('KRW');
	fixture.hold('KRW:USD');
	await choose(page, 'USD');
	await panel.getByRole('button', { name: '취소', exact: true }).click();
	await expect(currency).toHaveText('KRW');
	await expect(currency).toHaveAttribute('aria-busy', 'false');
	const olderResponse = page.waitForResponse(response => response.url().includes('/api/currencies/conversion?') && new URL(response.url()).searchParams.get('to') === 'USD');
	fixture.release('KRW:USD');
	await olderResponse;
	await expect(currency).toHaveText('KRW');
	await expect(amount).toHaveText('₩150만');
	await expect(panel.locator('[data-crm-view-currency]').getByRole('status')).toHaveCount(0);
});

test('currency loading and retry remain visible on a narrow screen', async ({ page }, testInfo) => {
	await page.setViewportSize({ width: 390, height: 844 });
	const fixture = new CurrencyFixture();
	const { panel, currency, amount } = await openDeals(page, fixture);
	await expect(currency).toHaveText('KRW');
	fixture.hold('KRW:USD');
	fixture.refused.add('KRW:USD');
	await choose(page, 'USD');
	await expect(currency).toBeInViewport();
	await expect(panel.getByRole('status')).toBeInViewport();
	await expect(amount).toHaveText('₩150만');
	await page.screenshot({ path: testInfo.outputPath('currency-pending-mobile.png') });
	fixture.release('KRW:USD');
	const retry = panel.getByRole('button', { name: '다시 시도', exact: true });
	await expect(retry).toBeInViewport();
	await expect(panel.getByRole('status')).toBeInViewport();
	await page.screenshot({ path: testInfo.outputPath('currency-error-mobile.png') });
	fixture.refused.delete('KRW:USD');
	await retry.click();
	await expect(currency).toHaveText('USD');
	await expect(amount).toHaveText('$1.5천');
});
