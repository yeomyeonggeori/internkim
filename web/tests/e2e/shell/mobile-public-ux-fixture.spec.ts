import { expect, test } from '@playwright/test';
import { expectTouchTarget } from '../support/touch-target';
import { mockBuzzDisabled } from '../messenger/buzz-test-routes';

for (const width of [320, 390, 1280]) {
	test(`claim recovery actions stay compact at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 844 });
		await mockBuzzDisabled(page);
		await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
		await page.route('**/api/auth/claim', route => route.fulfill({ json: {} }));
		await page.goto('/auth/claim');
		await page.getByRole('textbox', { name: '이메일', exact: true }).fill('sample@example.com');
		await page.locator('button[type="submit"]').click();
		await expect(page.locator('[data-slot="input-otp-slot"]')).toHaveCount(8);
		const confirm = page.getByRole('button', { name: '확인', exact: true });
		const resend = page.getByRole('button', { name: '다시 보내기', exact: true });
		const another = page.getByRole('button', { name: '다른 이메일 쓰기', exact: true });
		if (width < 640) {
			const [primary, left, right] = await Promise.all([confirm.boundingBox(), resend.boundingBox(), another.boundingBox()]);
			expect(primary && left && right).toBeTruthy();
			expect(Math.abs(left!.y - right!.y)).toBeLessThan(1);
			expect(left!.y).toBeGreaterThan(primary!.y + primary!.height);
			expect(right!.x).toBeGreaterThan(left!.x + left!.width);
			await expectTouchTarget(resend);
			await expectTouchTarget(another);
		}
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		const output = process.env.MOBILE_UX_SCREENSHOTS;
		if (output) await page.screenshot({ path: `${output}/claim-compact-${width}.png` });
		await another.click();
		await expect(page.getByRole('textbox', { name: '이메일', exact: true })).toBeVisible();
	});
}

for (const width of [320, 360, 390, 568, 1280]) {
	test(`public forms and shared categories fit ${width}px`, async ({ page }) => {
		test.setTimeout(90_000);
		await page.setViewportSize({ width, height: width === 568 ? 320 : 844 });
		await mockBuzzDisabled(page);
		await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
		await page.route('**/api/auth/claim', route => route.fulfill({ json: {} }));
		await page.goto('/auth/claim');
		await page.getByRole('textbox', { name: '이메일', exact: true }).fill('sample-with-a-long-address@example.com');
		await page.locator('button[type="submit"]').click();
		await expect(page.locator('[data-slot="input-otp-slot"]')).toHaveCount(8);
		await capture('claim-code');
		await page.goto('/start');
		await page.getByRole('textbox', { name: '회사 이름', exact: true }).fill('Sample Company With A Long Name');
		await page.getByRole('textbox', { name: '내 이름', exact: true }).fill('이샘플');
		await capture('start');
		await page.goto('/share/invitations/00000000-0000-4000-8000-000000000001');
		await expect(page.getByRole('button', { name: 'Send verification code' })).toBeVisible();
		await capture('share-invitation');
		await page.route('**/api/v1/data-room/00000000-0000-4000-8000-000000000002', route => route.fulfill({ json: {
			categories: [{ code: 'C', parent: null, name: 'A long category label that should remain readable', name_ko: '법인 자료', description: '', slug: 'corporate' }],
			documents: [{ id: 'document-sample', title: '공유 문서의 긴 제목과 상세 설명도 좁은 화면에서 읽을 수 있습니다', summary: '자료실 미리보기 검증', category_code: 'C', document_date: null, status: null, extension: 'pdf' }],
			company: {
				profile: { brandName: { ko: '샘플' }, slogan: { ko: '좁은 화면에서도 회사 소개가 한눈에 읽힙니다' }, capital: { amount: 500000000, currency: 'KRW' }, foundedDate: '2020-01-02' },
				metrics: [
					{ metric: 'annualRevenue', period: '2024', amount: { amount: 900000000, currency: 'KRW' } },
					{ metric: 'annualRevenue', period: '2025', amount: { amount: 1200000000, currency: 'KRW' } }
				],
				records: [{ category: 'funding', date: '2025-03-01', title: '시드 투자 유치', attributes: { leadInvestor: 'ABC Ventures' } }]
			},
			viewer: null
		} }));
		await page.goto('/share/00000000-0000-4000-8000-000000000002');
		await page.getByRole('tab', { name: /^(Documents|자료)$/ }).click();
		await expect(page.getByRole('button', { name: /공유 문서의 긴 제목/ })).toBeVisible();
		if (width < 768) {
			await page.getByRole('button', { name: /^(Folders|폴더)$/ }).click();
			await page.getByRole('option').last().click();
			await expect(page.getByRole('listbox')).toBeHidden();
		}
		await capture('shared-room');
		await page.getByRole('tab', { name: /^(Company|회사 소개)$/ }).click();
		await expect(page.getByText('시드 투자 유치')).toBeVisible();
		await capture('shared-room-company');
		await page.goto('/oauth/consent');
		await expect(page.locator('main')).toBeVisible();
		await capture('consent-sign-in');
		async function capture(name: string) {
			expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
			const output = process.env.MOBILE_UX_SCREENSHOTS;
			if (output) await page.screenshot({ path: `${output}/${name}-${width}.png` });
		}
	});
}
