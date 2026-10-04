import { expect, test, type Page } from '@playwright/test';

const facts = [
	{ factID: 'fact-preference', originID: 'origin-preference', scopeType: 'person', isStatic: true, content: '금요일 오후에 회고하는 것을 선호한다.', importance: 4, storageStrength: 1.2, createdAt: '2026-08-01T09:00:00Z', lastRecalledAt: '2026-08-20T09:00:00Z', triggerPhrases: [] },
	{ factID: 'fact-language', originID: 'origin-language', scopeType: 'circle', scopeID: 'member', isStatic: true, content: '한국어로 답변받는 것을 선호한다.', importance: 3, storageStrength: 1, createdAt: '2026-08-02T09:00:00Z', triggerPhrases: [] },
	{ factID: 'fact-previous', originID: 'origin-previous', scopeType: 'person', isStatic: false, content: '예전에는 월요일 오전 회고를 선호했다.', occurredAt: '2025-08-01T09:00:00Z', validUntil: '2026-01-01T09:00:00Z', importance: 2, storageStrength: 1, createdAt: '2025-08-01T09:00:00Z', triggerPhrases: [] }
];

function factsResponse(returnedFacts = facts) {
	return {
		personID: 'tester',
		layers: [{ scopeType: 'person', scopeID: 'tester' }, { scopeType: 'circle', scopeID: 'member' }, { scopeType: 'workspace' }],
		index: { embeddingModel: 'baai/bge-m3', current: returnedFacts.length, stale: 0 },
		facts: returnedFacts
	};
}

async function prepareMemoryPage(page: Page): Promise<void> {
	await page.route('**/admin/api/session', (route) => route.fulfill({ json: { email: 'tester@example.com' } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.goto('/');
	await expect.poll(async () => page.evaluate(async () => (await fetch('/auth/session')).json())).toMatchObject({ authenticated: true });
}

test.describe('memory workbench', () => {
	test('filters the loaded facts in the browser and clears back to all memories', async ({ page }) => {
		await prepareMemoryPage(page);
		let requestCount = 0;
		await page.route('**/memory/api/facts?**', async (route) => {
			requestCount += 1;
			await route.fulfill({ json: factsResponse() });
		});
		await page.goto('/memory/');
		await expect(page.locator('#memory-search')).toBeVisible();
		await expect(page.getByRole('region', { name: '서클 · member' })).toBeVisible();
		await expect(page.getByRole('main').getByRole('tab')).toHaveCount(0);
		await page.locator('#memory-search').fill('금요일');
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ })).toBeVisible();
		await expect(page.getByText('한국어로 답변받는 것을 선호한다.')).toBeHidden();
		await expect(page.getByText('검색 결과')).toBeVisible();
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await expect(page.getByLabel('저장된 기억').getByText('4/5')).toBeVisible();
		await page.screenshot({ path: '.artifacts/memory-ui/workbench-desktop-detail.png', fullPage: true });
		await page.getByRole('button', { name: '검색 지우기', exact: true }).click();
		await expect(page.locator('#memory-search')).toHaveValue('');
		await expect(page.getByText('현재 기억')).toBeVisible();
		await expect(page.getByText('한국어로 답변받는 것을 선호한다.')).toBeVisible();
		expect(requestCount).toBe(1);
	});

	test('reloads on refresh and exposes previous memories explicitly', async ({ page }) => {
		await prepareMemoryPage(page);
		let requestCount = 0;
		await page.route('**/memory/api/facts?**', async (route) => {
			requestCount += 1;
			await route.fulfill({ json: factsResponse() });
		});
		await page.goto('/memory/');
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ })).toBeVisible();
		await page.getByRole('main').getByRole('button', { name: '새로고침' }).click();
		await expect.poll(() => requestCount).toBeGreaterThanOrEqual(2);
		await expect(page.getByText('예전에는 월요일 오전 회고를 선호했다.')).toBeHidden();
		await page.getByRole('checkbox', { name: '지난 기억 포함', exact: true }).check();
		await expect(page.getByText('예전에는 월요일 오전 회고를 선호했다.')).toBeVisible();
	});

	test('forgets one exact fact with a reason and drops it from the list', async ({ page }) => {
		await prepareMemoryPage(page);
		await page.route('**/memory/api/facts?**', (route) => route.fulfill({ json: factsResponse() }));
		const forgets: { path: string; body: unknown }[] = [];
		await page.route('**/memory/api/facts/forget', async (route) => {
			forgets.push({ path: new URL(route.request().url()).pathname, body: route.request().postDataJSON() });
			await route.fulfill({ json: { forgottenFactIDs: ['fact-preference'] } });
		});
		await page.goto('/memory/');
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await page.getByRole('button', { name: '잊기' }).click();
		await page.getByLabel('이유 (선택)').fill('팀이 바뀜');
		await page.getByRole('form', { name: '기억 잊기' }).getByRole('button', { name: '잊기' }).click();
		await expect.poll(() => forgets.length).toBe(1);
		expect(forgets[0]).toEqual({ path: '/memory/api/facts/forget', body: { factIDs: ['fact-preference'], reason: '팀이 바뀜' } });
		await expect(page.getByText('금요일 오후에 회고하는 것을 선호한다.')).toHaveCount(0);
		await expect(page.getByText('한국어로 답변받는 것을 선호한다.')).toBeVisible();
	});

	test('keeps failure distinct from a successful empty state', async ({ page }) => {
		await prepareMemoryPage(page);
		await page.route('**/memory/api/facts?**', (route) => route.fulfill({ status: 502, body: 'private backend detail' }));
		await page.goto('/memory/');
		await expect(page.getByRole('main').getByRole('button', { name: '새로고침' })).toBeVisible();
		await expect(page.getByRole('alert')).toContainText('기억을 불러오지 못했습니다.');
		await expect(page.getByText('아직 볼 수 있는 기억이 없습니다.')).toHaveCount(0);
		await expect(page.getByText('private backend detail')).toHaveCount(0);
	});

	for (const width of [320, 375, 414, 768, 1280]) {
		test(`has no horizontal overflow at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 844 });
			await prepareMemoryPage(page);
			await page.route('**/memory/api/facts?**', (route) => route.fulfill({ json: factsResponse() }));
			await page.goto('/memory/');
			await expect(page.locator('#memory-search')).toBeVisible();
			await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
			await page.screenshot({ path: `.artifacts/memory-ui/workbench-${width}.png`, fullPage: true });
		});
	}

	test('returns from mobile detail to the memory list', async ({ page }) => {
		await page.setViewportSize({ width: 375, height: 844 });
		await prepareMemoryPage(page);
		await page.route('**/memory/api/facts?**', (route) => route.fulfill({ json: factsResponse() }));
		await page.goto('/memory/');
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await expect(page.getByLabel('저장된 기억')).toBeVisible();
		await expect(page.getByLabel('저장된 기억').getByText('마지막으로 떠올린 때')).toBeVisible();
		await page.screenshot({ path: '.artifacts/memory-ui/workbench-mobile-detail.png', fullPage: true });
		await page.getByRole('button', { name: '기억 목록', exact: true }).click();
		await expect(page.getByLabel('저장된 기억')).toBeHidden();
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ })).toBeVisible();
	});

	test('narrows to one layer and shows what a question would bring to mind', async ({ page }) => {
		await prepareMemoryPage(page);
		await page.route('**/memory/api/facts?**', (route) => route.fulfill({ json: factsResponse() }));
		let recallQuery = '';
		await page.route('**/memory/api/recall?**', async (route) => {
			recallQuery = new URL(route.request().url()).searchParams.get('query') ?? '';
			await route.fulfill({ json: { facts: [{ factID: 'fact-language', scopeType: 'circle', scopeID: 'member', content: '한국어로 답변받는 것을 선호한다.' }] } });
		});
		await page.goto('/memory/');

		const layers = page.getByRole('complementary', { name: '기억의 층' });
		await layers.getByRole('button', { name: /서클 · member/ }).click();
		await expect(page.getByRole('button', { name: /한국어로 답변받는/ })).toBeVisible();
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ })).toHaveCount(0);

		await page.getByRole('textbox', { name: '떠올려 보기' }).fill('답변 언어');
		await page.getByRole('button', { name: '떠올리기' }).click();
		await expect.poll(() => recallQuery).toBe('답변 언어');
		await page.getByRole('list', { name: '떠올려 보기' }).getByRole('button', { name: /한국어로 답변받는/ }).click();
		await expect(page.getByLabel('저장된 기억').getByText('서클 · member')).toBeVisible();
	});
});
