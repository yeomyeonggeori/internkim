import { expect, test, type Page } from '@playwright/test';

const sourcePrompt = '사용자는 금요일 오후에 회고를 선호합니다.';
const facts = [
	{ factID: 'fact-preference', scopeType: 'user', namespaceID: 'person:tester', content: '금요일 오후에 회고하는 것을 선호한다.', sourceKind: 'fact', sourceEpisodeIDs: ['episode-preference'], recordedAt: '2026-08-01T09:00:00Z', validAt: '2026-08-01T09:00:00Z' },
	{ factID: 'fact-language', scopeType: 'user', namespaceID: 'person:tester', content: '한국어로 답변받는 것을 선호한다.', sourceKind: 'fact', sourceEpisodeIDs: ['episode-language'], recordedAt: '2026-08-02T09:00:00Z', validAt: '2026-08-02T09:00:00Z' },
	{ factID: 'fact-previous', scopeType: 'user', namespaceID: 'person:tester', content: '예전에는 월요일 오전 회고를 선호했다.', sourceKind: 'fact', sourceEpisodeIDs: ['episode-previous'], recordedAt: '2025-08-01T09:00:00Z', validAt: '2025-08-01T09:00:00Z', invalidAt: '2026-01-01T09:00:00Z' }
];

function graphResponse(query = '', content = facts[0].content, returnedFacts = facts) {
	return {
		health: { configured: true, reachable: true },
		retrieval: { query, complete: true, limit: 120 },
		namespaces: [{ namespaceID: 'person:tester', scopeType: 'user', scopePersonID: 'tester' }],
		episodes: [
			{ episodeID: 'episode-preference', platform: 'mattermost', prompt: sourcePrompt, occurredAt: '2026-08-01T08:55:00Z', namespaceIDs: ['person:tester'] },
			{ episodeID: 'episode-language', platform: 'mattermost', prompt: '한국어 응답을 부탁드립니다.', occurredAt: '2026-08-02T08:55:00Z', namespaceIDs: ['person:tester'] },
			{ episodeID: 'episode-previous', platform: 'mattermost', prompt: '월요일 회고가 좋습니다.', occurredAt: '2025-08-01T08:55:00Z', namespaceIDs: ['person:tester'] }
		],
		facts: returnedFacts.map((fact) => fact.factID === 'fact-preference' ? { ...fact, content } : fact),
		nodes: [],
		edges: []
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
	test('runs semantic search in the single browser and clears back to all memories', async ({ page }) => {
		await prepareMemoryPage(page);
		const queries: string[] = [];
		await page.route('**/memory/api/graph**', async (route) => {
			const query = new URL(route.request().url()).searchParams.get('query') ?? '';
			queries.push(query);
			await route.fulfill({ json: graphResponse(query, facts[0].content, query ? [facts[0]] : facts) });
		});
		await page.goto('/memory/');
		await expect(page.locator('#memory-search')).toBeVisible();
		await expect(page.getByRole('main').getByRole('tab')).toHaveCount(0);
		await page.locator('#memory-search').fill('금요일 회고 선호');
		await page.getByRole('button', { name: '검색', exact: true }).click();
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ })).toBeVisible();
		await expect(page.locator('#memory-search')).toHaveValue('금요일 회고 선호');
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await expect(page.getByText(sourcePrompt)).toBeVisible();
		await page.screenshot({ path: '.artifacts/memory-ui/workbench-desktop-source.png', fullPage: true });
		await page.getByRole('button', { name: '검색 지우기', exact: true }).click();
		await expect(page.locator('#memory-search')).toHaveValue('');
		await expect(page.getByText('현재 기억')).toBeVisible();
		await expect(page.getByText('한국어로 답변받는 것을 선호한다.')).toBeVisible();
		expect(queries).toContain('금요일 회고 선호');
	});

	test('keeps search on refresh and exposes previous memories explicitly', async ({ page }) => {
		await prepareMemoryPage(page);
		const queries: string[] = [];
		await page.route('**/memory/api/graph**', async (route) => {
			const query = new URL(route.request().url()).searchParams.get('query') ?? '';
			queries.push(query);
			await route.fulfill({ json: graphResponse(query) });
		});
		await page.goto('/memory/');
		await page.locator('#memory-search').fill('회고');
		await page.getByRole('button', { name: '검색', exact: true }).click();
		await page.getByRole('main').getByRole('button', { name: '새로고침' }).click();
		await expect.poll(() => queries.filter((query) => query === '회고').length).toBeGreaterThanOrEqual(2);
		await expect(page.getByText('예전에는 월요일 오전 회고를 선호했다.')).toBeHidden();
		await page.getByRole('checkbox', { name: '이전 기억 포함', exact: true }).check();
		await expect(page.getByText('예전에는 월요일 오전 회고를 선호했다.')).toBeVisible();
	});

	test('edits and deletes one exact fact', async ({ page }) => {
		await prepareMemoryPage(page);
		let content = facts[0].content;
		let deleted = false;
		await page.route('**/memory/api/graph**', (route) => route.fulfill({ json: deleted ? graphResponse('', content, facts.slice(1)) : graphResponse('', content) }));
		const mutations: { path: string; body: unknown }[] = [];
		await page.route('**/memory/api/facts/**', async (route) => {
			mutations.push({ path: new URL(route.request().url()).pathname, body: route.request().postDataJSON() });
			if (route.request().url().endsWith('/update')) content = '금요일 오후 회고를 선호한다.';
			if (route.request().url().endsWith('/delete')) deleted = true;
			await route.fulfill({ json: { updated: true, deleted: true } });
		});
		await page.goto('/memory/');
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await page.getByRole('button', { name: /수정/ }).click();
		await page.getByLabel('수정').fill('금요일 오후 회고를 선호한다.');
		await page.getByRole('button', { name: '저장' }).click();
		await expect.poll(() => mutations.length).toBe(1);
		expect(mutations[0]).toEqual({ path: '/memory/api/facts/update', body: { factID: facts[0].factID, namespaceID: facts[0].namespaceID, content: '금요일 오후 회고를 선호한다.' } });
		await page.getByRole('button', { name: /삭제/ }).click();
		await page.getByRole('button', { name: '삭제', exact: true }).last().click();
		await expect.poll(() => mutations.length).toBe(2);
		expect(mutations[1]).toEqual({ path: '/memory/api/facts/delete', body: { factID: facts[0].factID, namespaceID: facts[0].namespaceID } });
		await expect(page.getByText('금요일 오후 회고를 선호한다.')).toHaveCount(0);
	});

	test('keeps failure distinct from a successful empty state', async ({ page }) => {
		await prepareMemoryPage(page);
		await page.route('**/memory/api/graph**', (route) => route.fulfill({ status: 502, body: 'private backend detail' }));
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
			await page.route('**/memory/api/graph**', (route) => route.fulfill({ json: graphResponse() }));
			await page.goto('/memory/');
			await expect(page.locator('#memory-search')).toBeVisible();
			await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
			await page.screenshot({ path: `.artifacts/memory-ui/workbench-${width}.png`, fullPage: true });
		});
	}

	test('returns from mobile detail to the memory list', async ({ page }) => {
		await page.setViewportSize({ width: 375, height: 844 });
		await prepareMemoryPage(page);
		await page.route('**/memory/api/graph**', (route) => route.fulfill({ json: graphResponse() }));
		await page.goto('/memory/');
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await expect(page.getByLabel('저장된 기억')).toBeVisible();
		await expect(page.getByText(sourcePrompt)).toBeVisible();
		await page.screenshot({ path: '.artifacts/memory-ui/workbench-mobile-detail.png', fullPage: true });
		await page.getByRole('button', { name: '기억 목록', exact: true }).click();
		await expect(page.getByLabel('저장된 기억')).toBeHidden();
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ })).toBeVisible();
	});
});
