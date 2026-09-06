import { expect, test, type Page } from '@playwright/test';

const sourcePrompt = '사용자는 금요일 오후에 회고를 선호합니다.';
const fact = {
	factID: 'fact-preference',
	scopeType: 'user',
	namespaceID: 'person:tester',
	content: '금요일 오후에 회고하는 것을 선호한다.',
	sourceKind: 'fact',
	sourceEpisodeIDs: ['episode-preference'],
	recordedAt: '2026-08-01T09:00:00Z',
	validAt: '2026-08-01T09:00:00Z'
};

function graphResponse(content = fact.content) {
	return {
		health: { configured: true, reachable: true },
		retrieval: { query: '', complete: true, limit: 120 },
		namespaces: [{ namespaceID: 'person:tester', scopeType: 'user', scopePersonID: 'tester' }],
		episodes: [{ episodeID: 'episode-preference', platform: 'mattermost', prompt: sourcePrompt, occurredAt: '2026-08-01T08:55:00Z', namespaceIDs: ['person:tester'] }],
		facts: [{ ...fact, content }],
		nodes: [],
		edges: []
	};
}

async function prepareMemoryPage(page: Page): Promise<void> {
	await page.route('**/admin/api/session', (route) => route.fulfill({ json: { email: 'tester@example.com' } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.goto('/');
	const session = await page.evaluate(async () => {
		const response = await fetch('/auth/session');
		return { ok: response.ok, body: await response.json() };
	});
	expect(session.ok).toBe(true);
	expect(session.body).toMatchObject({ authenticated: true });
}

test.describe('memory workbench', () => {
	test('submits semantic search and shows the returned source', async ({ page }) => {
		await prepareMemoryPage(page);
		const queries: string[] = [];
		await page.route('**/memory/api/graph**', async (route) => {
			const requestURL = new URL(route.request().url());
			queries.push(requestURL.searchParams.get('query') ?? '');
			await route.fulfill({ json: { ...graphResponse(), retrieval: { query: queries.at(-1) ?? '', complete: true, limit: 120 } } });
		});
		await page.goto('/memory/');
		await page.getByRole('tab', { name: '기억 검색' }).click();
		await page.locator('#memory-search').fill('금요일 회고 선호');
		await page.getByRole('button', { name: '검색', exact: true }).click();
		await expect(page.getByRole('button', { name: /금요일 오후에 회고/ }).last()).toBeVisible();
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await expect(page.getByText(sourcePrompt)).toBeVisible();
		await page.screenshot({ path: '.artifacts/memory-ui/workbench-desktop-source.png', fullPage: true });
		expect(queries).toContain('금요일 회고 선호');
	});

	test('edits and deletes one exact fact', async ({ page }) => {
		await prepareMemoryPage(page);
		let content = fact.content;
		let deleted = false;
		await page.route('**/memory/api/graph**', (route) => route.fulfill({ json: deleted ? { ...graphResponse(content), facts: [] } : graphResponse(content) }));
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
		await page.getByLabel('수정').fill(content);
		await page.getByRole('button', { name: '저장' }).click();
		await expect.poll(() => mutations.length).toBe(1);
		expect(mutations[0]).toEqual({ path: '/memory/api/facts/update', body: { factID: fact.factID, namespaceID: fact.namespaceID, content: fact.content } });
		await page.getByRole('button', { name: /삭제/ }).click();
		await page.getByRole('button', { name: '삭제', exact: true }).last().click();
		await expect.poll(() => mutations.length).toBe(2);
		expect(mutations[1]).toEqual({ path: '/memory/api/facts/delete', body: { factID: fact.factID, namespaceID: fact.namespaceID } });
		await expect(page.getByText('금요일 오후 회고를 선호한다.')).toHaveCount(0);
	});

	test('keeps the failure state actionable without showing an empty success state', async ({ page }) => {
		await prepareMemoryPage(page);
		await page.route('**/memory/api/graph**', (route) => route.fulfill({ status: 502, body: 'private backend detail' }));
		await page.goto('/memory/');
		await expect(page.getByRole('button', { name: '새로고침' })).toBeVisible();
		await expect(page.getByText('아직 볼 수 있는 기억이 없습니다.')).toHaveCount(0);
	});

	test('opens selected detail on a narrow viewport', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await prepareMemoryPage(page);
		await page.route('**/memory/api/graph**', (route) => route.fulfill({ json: graphResponse() }));
		await page.goto('/memory/');
		await page.getByRole('button', { name: /금요일 오후에 회고/ }).click();
		await expect(page.getByLabel('저장된 기억')).toBeVisible();
		await expect(page.getByText(sourcePrompt)).toBeVisible();
		await page.screenshot({ path: '.artifacts/memory-ui/workbench-mobile-detail.png', fullPage: true });
	});
});
