import { expect, type Page, test } from '@playwright/test';

const memoryGraphFixture = {
	health: { configured: true, reachable: true },
	namespaces: [{ namespaceID: 'workspace-memory', scopeType: 'workspace', episodeCount: 12 }],
	episodes: Array.from({ length: 8 }, (_, index) => ({ episodeID: `episode-${index}` })),
	facts: Array.from({ length: 10 }, (_, index) => ({
		factID: `fact-${index}`,
		scopeType: 'workspace',
		namespaceID: 'workspace-memory',
		content: `Memory fact ${index}`
	})),
	nodes: [
		{ nodeID: 'namespace', label: 'workspace-memory', kind: 'namespace', scopeType: 'workspace' },
		...Array.from({ length: 12 }, (_, index) => ({
			nodeID: `fact-${index}`,
			label: `Fact ${index}`,
			kind: 'fact',
			scopeType: 'workspace'
		}))
	],
	edges: Array.from({ length: 12 }, (_, index) => ({
		sourceID: 'namespace',
		targetID: `fact-${index}`,
		weight: (index % 3) + 1
	}))
};

type GraphMetrics = {
	bodyScrollWidth: number;
	canvas: GraphRect | null;
	container: GraphRect | null;
	hasHorizontalOverflow: boolean;
	viewportWidth: number;
};

type GraphRect = {
	height: number;
	width: number;
};

test.describe('memory graph', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com' } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
		await page.route('**/memory/api/graph**', async (route) => {
			await route.fulfill({ json: memoryGraphFixture });
		});
	});

	test('keeps graph canvas synced with its container after viewport resize', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/memory/');
		await page.waitForSelector('canvas');

		await expect.poll(async () => graphMetrics(page)).toMatchObject({
			bodyScrollWidth: 1280,
			hasHorizontalOverflow: false,
			viewportWidth: 1280
		});

		expectCanvasToMatchContainer(await graphMetrics(page));

		await page.setViewportSize({ width: 390, height: 844 });

		await expect.poll(async () => graphMetrics(page)).toMatchObject({
			bodyScrollWidth: 390,
			hasHorizontalOverflow: false,
			viewportWidth: 390
		});

		expectCanvasToMatchContainer(await graphMetrics(page));
	});
});

async function graphMetrics(page: Page): Promise<GraphMetrics> {
	return page.evaluate(() => {
		const canvas = document.querySelector('canvas');
		const container = canvas?.parentElement ?? null;
		return {
			bodyScrollWidth: document.body.scrollWidth,
			canvas: canvas?.getBoundingClientRect().toJSON() ?? null,
			container: container?.getBoundingClientRect().toJSON() ?? null,
			hasHorizontalOverflow: document.body.scrollWidth > window.innerWidth,
			viewportWidth: window.innerWidth
		};
	});
}

function expectCanvasToMatchContainer(metrics: GraphMetrics): void {
	expect(metrics.canvas).not.toBeNull();
	expect(metrics.container).not.toBeNull();
	expect(metrics.canvas?.width).toBe(metrics.container?.width);
	expect(metrics.canvas?.height).toBe(metrics.container?.height);
}
