import { expect, type Page, test } from '@playwright/test';

const memoryGraphNodeCount = 12;
const graphSizeTolerancePixel = 1;

const memoryGraphFixture = {
	health: { configured: true, reachable: true },
	namespaces: [
		{
			namespaceID: 'workspace-memory',
			scopeType: 'workspace',
			episodeCount: memoryGraphNodeCount
		}
	],
	episodes: Array.from({ length: memoryGraphNodeCount }, (_, index) => ({ episodeID: `episode-${index}` })),
	facts: Array.from({ length: memoryGraphNodeCount }, (_, index) => ({
		factID: `fact-${index}`,
		scopeType: 'workspace',
		namespaceID: 'workspace-memory',
		content: `Memory fact ${index}`
	})),
	nodes: [
		{ nodeID: 'namespace', label: 'workspace-memory', kind: 'namespace', scopeType: 'workspace' },
		...Array.from({ length: memoryGraphNodeCount }, (_, index) => ({
			nodeID: `fact-${index}`,
			label: `Fact ${index}`,
			kind: 'fact',
			scopeType: 'workspace'
		}))
	],
	edges: Array.from({ length: memoryGraphNodeCount }, (_, index) => ({
		sourceID: 'namespace',
		targetID: `fact-${index}`,
		weight: (index % 3) + 1
	}))
};

type GraphMetrics = {
	canvas: GraphSize | null;
	container: GraphSize | null;
	hasHorizontalOverflow: boolean;
	viewportWidth: number;
};

type GraphSize = {
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
			hasHorizontalOverflow: false,
			viewportWidth: 1280
		});

		await expectCanvasToMatchContainer(page);

		await page.setViewportSize({ width: 390, height: 844 });

		await expect.poll(async () => graphMetrics(page)).toMatchObject({
			hasHorizontalOverflow: false,
			viewportWidth: 390
		});

		await expectCanvasToMatchContainer(page);
	});
});

async function graphMetrics(page: Page): Promise<GraphMetrics> {
	return page.evaluate(() => {
		const canvas = document.querySelector('canvas');
		const container = canvas?.parentElement ?? null;
		const canvasRectangle = canvas?.getBoundingClientRect() ?? null;
		return {
			canvas: canvasRectangle
				? { height: canvasRectangle.height, width: canvasRectangle.width }
				: null,
			container: container ? { height: container.clientHeight, width: container.clientWidth } : null,
			hasHorizontalOverflow: document.body.scrollWidth > window.innerWidth,
			viewportWidth: window.innerWidth
		};
	});
}

async function expectCanvasToMatchContainer(page: Page): Promise<void> {
	await expect
		.poll(async () => calculateGraphSizeDifference(await graphMetrics(page)))
		.toBeLessThanOrEqual(graphSizeTolerancePixel);
}

function calculateGraphSizeDifference(metrics: GraphMetrics): number {
	if (!metrics.canvas || !metrics.container) return Number.MAX_SAFE_INTEGER;

	return Math.max(
		Math.abs(metrics.canvas.width - metrics.container.width),
		Math.abs(metrics.canvas.height - metrics.container.height)
	);
}
