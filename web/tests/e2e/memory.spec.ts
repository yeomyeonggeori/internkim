import { expect, test } from '@playwright/test';

const memoryFactsFixture = {
	personID: 'person-1',
	profile: { identityLines: ['이샘플은 플랫폼 팀 소속이다.'], currentLines: [] },
	facts: Array.from({ length: 12 }, (_, index) => ({
		factID: `fact-${index}`,
		episodeID: `episode-${index}`,
		ownerPersonID: 'person-1',
		circleIDs: ['member'],
		kind: 'fact',
		content: `Memory fact ${index}`,
		validFrom: `2026-06-${String(index + 1).padStart(2, '0')}T09:00:00Z`,
		reinforcementCount: 1
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

	test('lists the profile and the facts without horizontal overflow', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/memory/');

		await expect(page.getByText('이샘플은 플랫폼 팀 소속이다.')).toBeVisible();
		await expect(page.getByText('Memory fact 0')).toBeVisible();
		await expect(page.getByText('총 12개 중 12개 표시')).toBeVisible();
		await expect
			.poll(async () => page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth))
			.toBe(false);
		await expect(page.getByText('private backend detail')).toHaveCount(0);
	});

	test('reports a memory load failure without upstream detail', async ({ page }) => {
		await page.route('**/memory/api/facts**', async (route) => {
			await route.fulfill({ status: 502, body: 'private backend detail' });
		});
		await page.goto('/memory/');

		await expect(page.getByRole('alert')).toHaveText(/기억을 불러오지 못했습니다\.|Memory could not be loaded\./);
		await expect(page.getByText('private backend detail')).toHaveCount(0);
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

