import { expect, test } from '@playwright/test';

const memoryFactsFixture = {
	personID: 'person-1',
	layers: [{ scopeType: 'circle', scopeID: 'member' }, { scopeType: 'workspace' }],
	index: { embeddingModel: 'baai/bge-m3', current: 12, stale: 0 },
	facts: Array.from({ length: 12 }, (_, index) => ({
		factID: `fact-${index}`,
		originID: `origin-${index}`,
		scopeType: 'circle',
		scopeID: 'member',
		isStatic: true,
		content: `Memory fact ${index}`,
		importance: 3,
		storageStrength: 1,
		createdAt: `2026-06-${String(index + 1).padStart(2, '0')}T09:00:00Z`,
		triggerPhrases: []
	}))
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

test.describe('memory facts', () => {
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
		await page.route('**/memory/api/facts**', async (route) => {
			await route.fulfill({ json: memoryFactsFixture });
		});
		await page.route('**/memory/api/schedules**', async (route) => {
			await route.fulfill({ json: memoryScheduleFixture });
		});
	});

	test('lists the remembered facts with the circle they belong to', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/memory/');

		await expect(page.getByRole('button', { name: /Memory fact 0/ })).toBeVisible();
		await expect(page.getByRole('region', { name: '서클 · member' }).getByRole('button', { name: /Memory fact 0/ })).toBeVisible();
		await expect(page.getByRole('link', { name: '기억 지도' })).toHaveCount(0);
	});

	test('opens schedules from the memory page', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/memory/');

		await page.getByRole('link', { name: '예약 작업' }).click();
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
		await page.getByRole('link', { name: '예약 작업' }).click();

		await expect(page.getByText('예약 페이지 1')).toBeVisible();
		await expect(page.getByText('50개 중 1–25')).toBeVisible();

		await page.getByRole('button', { name: '다음' }).click();

		await expect(page.getByText('예약 페이지 2')).toBeVisible();
		await expect(page.getByText('50개 중 26–50')).toBeVisible();
	});

	test('shows an empty schedules state', async ({ page }) => {
		await page.route('**/memory/api/schedules**', async (route) => {
			await route.fulfill({ json: { schedules: [], count: 0 } });
		});
		await page.goto('/memory/');

		await page.getByRole('link', { name: '예약 작업' }).click();

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

		await page.getByRole('link', { name: '예약 작업' }).click();

		await expect(page.getByText('예약 작업을 불러오지 못했습니다.')).toBeVisible();
		await expect(page.getByText('private backend detail')).toHaveCount(0);
	});

	test('reports a memory load failure without upstream detail', async ({ page }) => {
		await page.route('**/memory/api/facts**', async (route) => {
			await route.fulfill({ status: 502, body: 'private backend detail' });
		});
		await page.goto('/memory/');

		await expect(page.getByRole('alert')).toHaveText(/기억을 불러오지 못했습니다\.|Memories could not be loaded\./);
		await expect(page.getByText('private backend detail')).toHaveCount(0);
	});

	test('formats schedule next run in the selected English locale', async ({ page }) => {
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await page.goto('/memory/');

		await page.getByRole('link', { name: 'Schedules' }).click();

		await expect(page.getByText('Jun').first()).toBeVisible();
		await expect(page.getByText('2026. 6. 9.')).toHaveCount(0);
	});
});

