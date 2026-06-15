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

const unavailableMemoryGraphFixture = {
	...memoryGraphFixture,
	health: { configured: false, reachable: false }
};

const memoryScheduleFixture = {
	schedules: [
		{
			taskScheduleID: 'schedule-daily-brief',
			creatorPersonID: 'user:person-1',
			executionMode: 'agent',
			kind: 'cron',
			cronExpression: '0 9 * * *',
			nextRunAt: '2099-06-09T00:00:00Z',
			expiresAt: '2099-06-10T09:00:00Z',
			createdAt: '2026-06-08T00:00:00Z',
			updatedAt: '2026-06-08T00:00:00Z',
			deliveryChannelID: 'channel-1',
			promptPreview: '팀 일정을 매일 오전에 알려주기',
			timeZone: 'Asia/Seoul'
		},
		{
			taskScheduleID: 'schedule-expired',
			creatorPersonID: 'user:person-1',
			executionMode: 'agent',
			kind: 'interval',
			intervalSecond: 3600,
			expiresAt: '2026-06-07T09:00:00Z',
			completedRunCount: 3,
			createdAt: '2026-06-01T00:00:00Z',
			updatedAt: '2026-06-07T09:00:00Z',
			deliveryChannelID: 'channel-1',
			promptPreview: '이미 만료된 예약',
			timeZone: 'Asia/Seoul'
		}
	],
	count: 2,
	totalCount: 2,
	checkedAt: '2026-06-08T00:00:00Z'
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
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
		await page.route('**/memory/api/graph**', async (route) => {
			await route.fulfill({ json: memoryGraphFixture });
		});
		await page.route('**/memory/api/schedules**', async (route) => {
			await route.fulfill({ json: memoryScheduleFixture });
		});
	});

	test('keeps graph canvas synced with its container after viewport resize', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/memory/');
		await page.waitForSelector('canvas');

		await expect(page.getByText('Memory fact 0')).toBeVisible();
		await expect(page.getByText('신뢰도 미제공').first()).toBeVisible();

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

	test('shows user schedules in the schedules tab', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/memory/');

		await page.getByRole('tab', { name: '예약 작업' }).click();
		const schedulesPanel = page.getByLabel('예약 작업');

		await expect(page.getByText('팀 일정을 매일 오전에 알려주기')).toBeVisible();
		await expect(schedulesPanel.getByText('유형', { exact: true })).toBeVisible();
		await expect(schedulesPanel.getByText('일정', { exact: true })).toBeVisible();
		await expect(schedulesPanel.getByText('종료일', { exact: true })).toBeVisible();
		await expect(schedulesPanel.getByText('상태', { exact: true })).toBeVisible();
		await expect(page.getByText('정기 반복')).toBeVisible();
		await expect(page.getByText('간격 반복')).toBeVisible();
		await expect(page.getByText('매일 오전 9:00')).toBeVisible();
		await expect(page.getByText('2099. 6. 9.')).toBeVisible();
		await expect(page.getByText('2099. 6. 10.')).toBeVisible();
		await expect(page.getByText('이미 만료된 예약')).toBeVisible();
		await expect(page.getByText('만료됨')).toBeVisible();
		await expect(page.getByRole('button', { name: /수정/ }).first()).toBeVisible();
		await expect(page.getByRole('button', { name: /삭제/ }).first()).toBeVisible();
	});

	test('pages through visible schedules', async ({ page }) => {
		await page.route('**/memory/api/schedules**', async (route) => {
			const requestURL = new URL(route.request().url());
			const pageNumber = Number(requestURL.searchParams.get('page') ?? '1');
			await route.fulfill({
				json: {
					schedules: [
						{
							taskScheduleID: `schedule-page-${pageNumber}`,
							creatorPersonID: `person-${pageNumber}`,
							executionMode: 'agent',
							kind: 'cron',
							cronExpression: '0 9 * * *',
							nextRunAt: '2099-06-09T00:00:00Z',
							createdAt: '2026-06-08T00:00:00Z',
							updatedAt: '2026-06-08T00:00:00Z',
							deliveryChannelID: 'channel-1',
							promptPreview: `예약 페이지 ${pageNumber}`,
							timeZone: 'Asia/Seoul'
						}
					],
					count: 1,
					totalCount: 50,
					page: pageNumber,
					pageSize: 25
				}
			});
		});
		await page.goto('/memory/');

		await page.getByRole('tab', { name: '예약 작업' }).click();

		await expect(page.getByText('예약 페이지 1')).toBeVisible();
		await expect(page.getByText('1-25 / 50')).toBeVisible();

		await page.getByRole('button', { name: '2' }).click();

		await expect(page.getByText('예약 페이지 2')).toBeVisible();
		await expect(page.getByText('26-50 / 50')).toBeVisible();
	});

	test('shows an empty schedules state', async ({ page }) => {
		await page.route('**/memory/api/schedules**', async (route) => {
			await route.fulfill({ json: { schedules: [], count: 0 } });
		});
		await page.goto('/memory/');

		await page.getByRole('tab', { name: '예약 작업' }).click();

		await expect(page.getByText('아직 예약 작업이 없습니다.')).toBeVisible();
	});

	test('shows a schedules failure state without raw backend details', async ({ page }) => {
		await page.route('**/memory/api/schedules**', async (route) => {
			await route.fulfill({
				status: 500,
				body: 'Traceback File "/opt/blueclaw/internal.py" RuntimeError: private backend detail'
			});
		});
		await page.goto('/memory/');

		await page.getByRole('tab', { name: '예약 작업' }).click();

		await expect(page.getByText('예약 작업을 불러오지 못했습니다.')).toBeVisible();
		await expect(page.getByText('private backend detail')).toHaveCount(0);
	});

	test('keeps memory graph health badges tied to graph health', async ({ page }) => {
		await page.route('**/memory/api/graph**', async (route) => {
			await route.fulfill({ json: unavailableMemoryGraphFixture });
		});
		await page.goto('/memory/');

		await expect(page.getByText('미설정')).toBeVisible();
		await expect(page.getByText('연결 불가')).toBeVisible();
	});

	test('formats schedule next run in the selected English locale', async ({ page }) => {
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await page.goto('/memory/');

		await page.getByRole('tab', { name: 'Schedules' }).click();

		await expect(page.getByText('Jun').first()).toBeVisible();
		await expect(page.getByText('2026. 6. 9.')).toHaveCount(0);
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
