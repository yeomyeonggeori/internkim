import { expect, test, type Page, type Route } from '@playwright/test';

test.describe('calendar draft popover', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarAPI(page);
	});

	test('opens a custom draft popover from the New button without posting a default title event', async ({ page }) => {
		let eventCreateCount = 0;
		await page.route('**/calendar/api/events', async (route) => {
			if (route.request().method() === 'POST') eventCreateCount += 1;
			await route.fulfill({
				json: {
					id: 'created-event',
					title: 'Created event',
					startISO: '2026-06-08T09:00:00.000Z',
					endISO: '2026-06-08T10:00:00.000Z',
					isAllDay: false
				}
			});
		});

		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('제목')).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeVisible();
		expect(eventCreateCount).toBe(0);
	});

	test('uses localized accessible labels for date and time fields', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		const popover = page.locator('.calendar-draft-popover');

		await expect(popover.getByLabel('시작 날짜')).toBeVisible();
		await expect(popover.getByLabel('종료 날짜')).toBeVisible();
		await expect(popover.getByLabel('시작 시간')).toBeVisible();
		await expect(popover.getByLabel('종료 시간')).toBeVisible();
	});

	test('posts the draft only after the popover is completed', async ({ page }) => {
		const postedPayloads: unknown[] = [];
		await page.route('**/calendar/api/events', async (route) => {
			const payload = route.request().postDataJSON() as Record<string, unknown>;
			postedPayloads.push(payload);
			await route.fulfill({
				json: {
					id: String(payload.eventID),
					title: String(payload.title),
					description: String(payload.description ?? ''),
					location: String(payload.location ?? ''),
					startISO: String(payload.startISO),
					endISO: String(payload.endISO),
					isAllDay: Boolean(payload.isAllDay),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			});
		});

		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		await page.getByLabel('제목').fill('디자인 리뷰');
		await page.getByLabel('장소').fill('회의실 A');
		await page.getByLabel('설명').fill('월간 디자인 점검');
		expect(postedPayloads).toHaveLength(0);

		await page.getByRole('button', { name: '완료' }).click();

		await expect.poll(() => postedPayloads.length).toBe(1);
		expect(postedPayloads[0]).toMatchObject({
			title: '디자인 리뷰',
			location: '회의실 A',
			description: '월간 디자인 점검',
			isAllDay: false
		});
		await expect(page.locator('.calendar-draft-popover')).toBeHidden();
	});

	test('saves titled drafts and cancels untitled drafts when clicking outside the popover', async ({ page }) => {
		const postedPayloads: unknown[] = [];
		await page.route('**/calendar/api/events', async (route) => {
			const payload = route.request().postDataJSON() as Record<string, unknown>;
			postedPayloads.push(payload);
			await route.fulfill({
				json: {
					id: String(payload.eventID),
					title: String(payload.title),
					description: String(payload.description ?? ''),
					location: String(payload.location ?? ''),
					startISO: String(payload.startISO),
					endISO: String(payload.endISO),
					isAllDay: Boolean(payload.isAllDay),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		for (const viewLabel of ['일', '주', '월']) {
			await page.getByRole('button', { name: viewLabel, exact: true }).click();
			await page.getByRole('button', { name: '새로 만들기' }).click();
			await page.getByLabel('제목').fill(`${viewLabel} 바깥 저장`);
			await clickOutsideDraftPopover(page);
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		}

		await expect.poll(() => postedPayloads.length).toBe(3);
		expect(postedPayloads.map((payload) => (payload as { title?: string }).title)).toEqual([
			'일 바깥 저장',
			'주 바깥 저장',
			'월 바깥 저장'
		]);

		for (const viewLabel of ['일', '주', '월']) {
			await page.getByRole('button', { name: viewLabel, exact: true }).click();
			await page.getByRole('button', { name: '새로 만들기' }).click();
			await clickOutsideDraftPopover(page);
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		}

		await expect.poll(() => postedPayloads.length).toBe(3);
	});

	test('disables completion when a timed draft end is not after the start', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		const popover = page.locator('.calendar-draft-popover');
		await page.getByLabel('제목').fill('시간 검증');
		await expect(popover.getByRole('button', { name: '완료' })).toBeEnabled();

		await popover.locator('input[type="time"]').nth(1).fill('09:00');

		await expect(popover.getByRole('button', { name: '완료' })).toBeDisabled();
	});

	test('keeps month single click as selection and opens the popover on double click', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		const dateCell = page.locator('.df-month-day-cell[data-date="2026-06-10"]');
		await dateCell.click();
		await expect(dateCell).toHaveClass(/month-selected-date/);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);

		await dateCell.dblclick();

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toBeVisible();
	});

	test('cancels a new draft without leaving a local event behind', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		await expect(page.locator('[data-event-id^="quick-"]')).toHaveCount(1);

		await page.locator('.draft-popover-footer').getByRole('button', { name: '취소' }).click();

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id^="quick-"]')).toHaveCount(0);
	});

	test('opens one all-day draft popover for a dragged month range', async ({ page }) => {
		const postedPayloads: unknown[] = [];
		await page.route('**/calendar/api/events', async (route) => {
			const payload = route.request().postDataJSON() as Record<string, unknown>;
			postedPayloads.push(payload);
			await route.fulfill({
				json: {
					id: String(payload.eventID),
					title: String(payload.title),
					description: String(payload.description ?? ''),
					location: String(payload.location ?? ''),
					startISO: String(payload.startISO),
					endISO: String(payload.endISO),
					isAllDay: Boolean(payload.isAllDay),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await dragBetweenCells(page, '2026-06-10', '2026-06-12');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeChecked();
		await page.getByLabel('제목').fill('워크숍');
		await page.getByRole('button', { name: '완료' }).click();

			await expect.poll(() => postedPayloads.length).toBe(1);
			expect(postedPayloads[0]).toMatchObject({
				title: '워크숍',
				isAllDay: true,
				startISO: '2026-06-10T00:00:00.000Z',
				endISO: '2026-06-13T00:00:00.000Z'
			});
		});

	test('opens existing events in edit mode and deletes them from the popover', async ({ page }) => {
		let deleteCount = 0;
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'existing-event',
							title: '기존 일정',
							description: '기존 설명',
							location: '회의실 B',
							startISO: '2026-06-10T09:00:00.000Z',
							endISO: '2026-06-10T10:00:00.000Z',
							isAllDay: false
						}
					]
				}
			});
		});
		await page.route('**/calendar/api/events/existing-event', async (route) => {
			if (route.request().method() === 'DELETE') deleteCount += 1;
			await route.fulfill({ json: {} });
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByText('기존 일정').click();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue('기존 일정');

		await page.getByRole('button', { name: '삭제' }).click();

		await expect.poll(() => deleteCount).toBe(1);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.getByText('기존 일정')).toHaveCount(0);
	});
});

async function routeCalendarAPI(page: Page): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({ json: { events: [] } });
	});
	await page.route('**/calendar/api/sync', async (route) => {
		await route.fulfill({
			json: {
				caldavURL: '',
				caldavUsername: '',
				caldavPassword: '',
				icsURL: ''
			}
		});
	});
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({ json: { connected: false, needsReauth: false } });
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/calendar/api/conflicts/*', async (route) => {
		await fulfillEmptyResponse(route);
	});
}

async function fulfillEmptyResponse(route: Route): Promise<void> {
	await route.fulfill({ json: {} });
}

async function waitForClientHydration(page: Page): Promise<void> {
	await page.waitForTimeout(1_000);
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
}

async function clickOutsideDraftPopover(page: Page): Promise<void> {
	const clickPoint = await page.evaluate(() => {
		const popover = document.querySelector('.calendar-draft-popover');
		const stage = document.querySelector('.calendar-stage');
		if (!(popover instanceof HTMLElement) || !(stage instanceof HTMLElement)) {
			throw new Error('Missing draft popover or calendar stage');
		}
		const stageRectangle = stage.getBoundingClientRect();
		const popoverRectangle = popover.getBoundingClientRect();
		const candidates = [
			{ x: stageRectangle.left + 24, y: stageRectangle.top + 72 },
			{ x: stageRectangle.right - 24, y: stageRectangle.top + 72 },
			{ x: stageRectangle.left + 24, y: stageRectangle.bottom - 24 },
			{ x: stageRectangle.left + stageRectangle.width / 2, y: stageRectangle.top + stageRectangle.height / 2 }
		];
		const point = candidates.find(
			(candidate) =>
				candidate.x < popoverRectangle.left ||
				candidate.x > popoverRectangle.right ||
				candidate.y < popoverRectangle.top ||
				candidate.y > popoverRectangle.bottom
		);
		if (!point) throw new Error('Could not find a point outside the draft popover');
		return point;
	});
	await page.mouse.click(clickPoint.x, clickPoint.y);
}

async function dragBetweenCells(page: Page, startDateKey: string, endDateKey: string): Promise<void> {
	const startBox = await page.locator(`.df-month-day-cell[data-date="${startDateKey}"]`).boundingBox();
	const endBox = await page.locator(`.df-month-day-cell[data-date="${endDateKey}"]`).boundingBox();
	if (!startBox || !endBox) throw new Error('month cells must be visible before dragging');
	await page.mouse.move(startBox.x + startBox.width / 2, startBox.y + startBox.height / 2);
	await page.mouse.down();
	await page.mouse.move(endBox.x + endBox.width / 2, endBox.y + endBox.height / 2, { steps: 8 });
	await page.mouse.up();
}
