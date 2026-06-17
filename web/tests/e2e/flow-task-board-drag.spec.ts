import { expect, type Locator, type Page, test } from '@playwright/test';

const requestedTaskID = '26W23-attendance-policy';
const scheduledTaskID = '26W23-mail-triage';
const flowDashboardTaskID = '26W23-flow-dashboard';
const marketScanTaskID = '26W23-market-scan';

test.describe('flow task board drag interactions', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('opens on the personal task board with compact filters', async ({ page }) => {
		await page.goto('/flow/');

		await expect(page.locator('[data-flow-active-tab="tasks"]')).toBeVisible();
		await expect(page.getByRole('button', { name: '김철수', exact: true })).toHaveCount(0);
		await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
		await expect(taskColumn(page, '진행')).toBeVisible();
		await expect(page.getByRole('button', { name: /필터/ })).toBeVisible();
		await expect(page.getByPlaceholder('내용, 목표, 참여자 검색')).toHaveCount(0);

		await page.getByRole('button', { name: /필터/ }).click();
		await expect(page.getByPlaceholder('내용, 목표, 참여자 검색')).toBeVisible();
		await expect(page.locator('[data-flow-filter-panel]').getByText('김철수')).toBeVisible();
	});

	test('moves cards across columns and preserves reordered cards after reload', async ({ page }) => {
		await openFlowBoard(page);

		await dragToLocator(taskCard(page, requestedTaskID), columnAppendTarget(page, '진행'));
		await expect(taskColumn(page, '진행').locator(`[data-flow-board-card="${requestedTaskID}"]`)).toBeVisible();

		await showUpperInsertionIndicator(page, taskCard(page, marketScanTaskID), taskCard(page, flowDashboardTaskID));
		await expect(insertionIndicator(page, '진행', flowDashboardTaskID)).toBeVisible();
		await expectInsertionSlotAbove(insertionIndicator(page, '진행', flowDashboardTaskID), taskCard(page, flowDashboardTaskID));

		await dragToUpperHalf(taskCard(page, marketScanTaskID), taskCard(page, flowDashboardTaskID));
		await expect(progressCardIDs(page)).resolves.toEqual([
			'26W23-customer-reply',
			marketScanTaskID,
			flowDashboardTaskID,
			'26W23-roadmap-review',
			requestedTaskID
		]);

		await page.reload();
		await openFlowBoard(page);

		await expect(progressCardIDs(page)).resolves.toEqual([
			'26W23-customer-reply',
			marketScanTaskID,
			flowDashboardTaskID,
			'26W23-roadmap-review',
			requestedTaskID
		]);
	});

	test('rolls back the card when drag save fails', async ({ page }) => {
		let shouldFail = true;
		await page.route('**/flow/api/tasks/move', async (route) => {
			const payload = flowTaskBoardMovePayload(route.request().postData());
			if (route.request().method() !== 'POST' || payload.taskID !== scheduledTaskID || !shouldFail) {
				await route.continue();
				return;
			}
			shouldFail = false;
			await new Promise((resolve) => setTimeout(resolve, 200));
			await route.fulfill({ status: 403, body: 'forbidden' });
		});

		await openFlowBoard(page);
		await dragToLocator(taskCard(page, scheduledTaskID), columnAppendTarget(page, '진행'));

		await expect(taskCard(page, scheduledTaskID)).toHaveAttribute('data-flow-board-pending', 'true');
		await expect(page.getByText('업무를 저장하지 못했습니다.')).toBeVisible();
		await expect(taskColumn(page, '예정').locator(`[data-flow-board-card="${scheduledTaskID}"]`)).toBeVisible();
		await expect(taskColumn(page, '진행').locator(`[data-flow-board-card="${scheduledTaskID}"]`)).toHaveCount(0);
	});

	test('keeps the board visible when reload fails after a saved board move', async ({ page }) => {
		await openFlowBoard(page);

		let shouldFailNextStateReload = true;
		await page.route('**/flow/api/state', async (route) => {
			if (!shouldFailNextStateReload) {
				await route.continue();
				return;
			}
			shouldFailNextStateReload = false;
			await route.fulfill({ status: 503, body: 'reload failed' });
		});

		await dragToLocator(taskCard(page, scheduledTaskID), columnAppendTarget(page, '진행'));

		await expect(page.getByText('업무 데이터를 불러오지 못했습니다.')).toBeVisible();
		await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
		await expect(taskColumn(page, '진행').locator(`[data-flow-board-card="${scheduledTaskID}"]`)).toBeVisible();
	});

	test('ignores external drag payloads that did not start from a board card', async ({ page }) => {
		let boardMoveRequestCount = 0;
		page.on('request', (request) => {
			const requestURL = new URL(request.url());
			if (request.method() === 'POST' && requestURL.pathname === '/flow/api/tasks/move') {
				boardMoveRequestCount += 1;
			}
		});

		await openFlowBoard(page);
		await page.evaluate((taskID) => {
			const dataTransfer = new DataTransfer();
			dataTransfer.setData('text/plain', taskID);
			const target = document.querySelector('[data-flow-board-drop-zone="진행"]');
			if (!target) throw new Error('progress drop zone was not found');
			target.dispatchEvent(new DragEvent('dragover', { bubbles: true, clientY: 4, dataTransfer }));
			target.dispatchEvent(new DragEvent('drop', { bubbles: true, clientY: 4, dataTransfer }));
		}, scheduledTaskID);

		await expect(taskColumn(page, '예정').locator(`[data-flow-board-card="${scheduledTaskID}"]`)).toBeVisible();
		await expect(taskColumn(page, '진행').locator(`[data-flow-board-card="${scheduledTaskID}"]`)).toHaveCount(0);
		await page.waitForTimeout(50);
		expect(boardMoveRequestCount).toBe(0);
	});

	test('moves visible cards by relative order while task filters are active', async ({ page }) => {
		await openFlowBoard(page);
		await page.getByRole('button', { name: /필터/ }).click();
		await page.getByPlaceholder('내용, 목표, 참여자 검색').fill('정리');
		await page.keyboard.press('Escape');

		await expect(taskCard(page, flowDashboardTaskID)).toBeVisible();
		await expect(taskCard(page, marketScanTaskID)).toHaveCount(0);
		await expect(taskCard(page, '26W23-roadmap-review')).toHaveAttribute('draggable', 'true');

		await dragToUpperHalf(taskCard(page, '26W23-roadmap-review'), taskCard(page, flowDashboardTaskID));
		await page.getByRole('button', { name: /필터/ }).click();
		await page.getByPlaceholder('내용, 목표, 참여자 검색').fill('');
		await page.keyboard.press('Escape');

		const cardIDs = await progressCardIDs(page);
		expect(cardIDs.indexOf('26W23-customer-reply')).toBeLessThan(cardIDs.indexOf('26W23-roadmap-review'));
		expect(cardIDs.indexOf('26W23-roadmap-review')).toBeLessThan(cardIDs.indexOf(flowDashboardTaskID));
	});

	test('keeps the selected week when a delayed board move finishes after week navigation', async ({ page }) => {
		let releaseSave: () => void = () => {};
		let shouldCountOldWeekReload = false;
		let oldWeekReloads = 0;
		const saveMayContinue = new Promise<void>((resolve) => {
			releaseSave = resolve;
		});
		await page.route('**/flow/api/summary?week=26W23', async (route) => {
			if (shouldCountOldWeekReload) oldWeekReloads += 1;
			await route.continue();
		});
		await page.route('**/flow/api/tasks/move', async (route) => {
			const payload = flowTaskBoardMovePayload(route.request().postData());
			if (route.request().method() !== 'POST' || payload.taskID !== scheduledTaskID) {
				await route.continue();
				return;
			}
			await saveMayContinue;
			await route.continue();
		});

		await openFlowBoard(page);
		await dragToLocator(taskCard(page, scheduledTaskID), columnAppendTarget(page, '진행'));
		await page.getByRole('button', { name: '다음 주', exact: true }).click();
		await expect(page.getByRole('button', { name: '날짜로 주차 이동' })).toContainText('6/8 - 6/14');
		shouldCountOldWeekReload = true;
		releaseSave();

		await page.waitForTimeout(300);
		expect(oldWeekReloads).toBe(0);
		await expect(page.getByRole('button', { name: '날짜로 주차 이동' })).toContainText('6/8 - 6/14');
		await expect(page).toHaveURL(/week=26W24/);
	});

	test('keeps the board horizontally scrollable on a narrow viewport', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await openFlowBoard(page);

		await expect.poll(async () => {
			return page.locator('[data-flow-board-scroll]').evaluate((element) => element.clientWidth > 0);
		}).toBe(true);
		await expect(page.locator('[data-flow-board-scroll]')).toHaveJSProperty('scrollLeft', 0);
		await expect.poll(async () => {
			return page.locator('[data-flow-board-scroll]').evaluate((element) => element.scrollWidth > element.clientWidth);
		}).toBe(true);
	});

	test('opens AI quick add as a bottom floating conversation panel', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);

		const closedLauncher = page.getByRole('button', { name: 'AI로 업무 추가', exact: true });
		const closedLauncherBox = await visibleBoundingBox(closedLauncher);
		await closedLauncher.click();

		const openLauncher = page.getByRole('button', { name: 'AI 업무 추가 닫기', exact: true });
		await expect(openLauncher).toBeVisible();
		await expect(openLauncher.locator('[data-flow-quick-add-closed-content]')).toBeAttached();
		await expect(openLauncher.locator('[data-flow-quick-add-open-content]')).toBeAttached();
		await expect.poll(async () => {
			const box = await openLauncher.boundingBox();
			return box?.width ?? 0;
		}).toBeLessThan(closedLauncherBox.width * 0.5);
		const openLauncherBox = await visibleBoundingBox(openLauncher);
		const panel = page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true });
		const panelBox = await visibleBoundingBox(panel);
		const quickAddInput = page.getByPlaceholder('예: 10분 회의');
		const viewportSize = page.viewportSize();
		if (!viewportSize) throw new Error('viewport size was not available');

		await expect(panel).toHaveCSS('transition-duration', '0.4s');
		await expect(quickAddInput).toBeFocused();
		expect(openLauncherBox.width).toBeLessThan(closedLauncherBox.width * 0.5);
		expect(Math.abs(openLauncherBox.x + openLauncherBox.width - (closedLauncherBox.x + closedLauncherBox.width))).toBeLessThan(4);
		expect(Math.abs(openLauncherBox.y + openLauncherBox.height - (closedLauncherBox.y + closedLauncherBox.height))).toBeLessThan(4);
		expect(panelBox.width).toBeGreaterThanOrEqual(390);
		expect(panelBox.width).toBeLessThanOrEqual(420);
		expect(panelBox.height).toBeGreaterThanOrEqual(490);
		expect(panelBox.height).toBeLessThanOrEqual(510);
		expect(panelBox.height).toBeGreaterThan(panelBox.width * 1.2);
		expect(panelBox.x + panelBox.width).toBeGreaterThan(viewportSize.width - 40);

		await quickAddInput.fill('새 업무');
		await page.keyboard.press('Tab');
		await expect(panel.getByRole('button', { name: 'AI로 업무 추가', exact: true })).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(openLauncher).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(quickAddInput).toBeFocused();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog', { name: 'AI로 업무 추가', exact: true })).toHaveCount(0);
		await expect(closedLauncher).toBeFocused();
		await expect(closedLauncher).toBeVisible();
	});

	test('shows definition autosave feedback after edits', async ({ page }) => {
		let releaseDefinitionsSave: () => void = () => {};
		const definitionsSaveMayContinue = new Promise<void>((resolve) => {
			releaseDefinitionsSave = resolve;
		});
		await page.route('**/flow/api/definitions', async (route) => {
			if (route.request().method() !== 'PUT') {
				await route.continue();
				return;
			}
			await definitionsSaveMayContinue;
			await route.continue();
		});

		await page.goto('/flow/');
		await page.getByRole('button', { name: '정의', exact: true }).click();
		await expect(page.getByText('추가와 삭제는 즉시 저장됩니다.')).toBeVisible();

		await page.getByPlaceholder('사업').fill('신규 사업');
		await page.getByRole('button', { name: '추가', exact: true }).first().click();
		await expect(page.getByText('저장 중...')).toBeVisible();
		releaseDefinitionsSave();
		await expect(page.getByText('저장됨')).toBeVisible();
	});

	test('explains why a non-participant task editor is read-only', async ({ page }) => {
		await useMemberFlowSession(page, 'designer@example.com', '이영희');
		await openFlowBoard(page);

		await taskCard(page, marketScanTaskID).click();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toBeVisible();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '업무 삭제', exact: true })).toHaveCount(0);
	});

	test('keeps assignment fields locked for a participant who is not the owner', async ({ page }) => {
		await useMemberFlowSession(page, 'engineer@example.com', '박민준');
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();
		await expect(page.getByPlaceholder('업무 내용')).toBeEnabled();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toBeVisible();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toHaveCount(0);
		await expect(page.getByRole('button', { name: /제거$/ })).toHaveCount(0);
	});

	test('keeps the owner participant locked while assignment fields are editable', async ({ page }) => {
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();

		await expect(page.getByRole('button', { name: '김철수 제거', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '박민준 제거', exact: true })).toBeVisible();
	});
});

async function openFlowBoard(page: Page): Promise<void> {
	await page.goto('/flow/');
	await expect(page.getByRole('button', { name: '업무', exact: true })).toBeVisible();
	await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
	await page.getByRole('button', { name: /필터/ }).click();
	await page.locator('[data-flow-filter-panel]').getByRole('button', { name: '전체 참여자', exact: true }).click();
	await page.keyboard.press('Escape');
	await expect(taskColumn(page, '진행')).toBeVisible();
}

async function useMemberFlowSession(page: Page, email: string, name: string): Promise<void> {
	await page.route('**/flow/api/state', async (route) => {
		const response = await route.fetch();
		const state: unknown = await response.json();
		if (!isUnknownRecord(state)) throw new Error('flow state response was not an object');
		await route.fulfill({
			response,
			json: {
				...state,
				currentUserEmail: email,
				currentUserName: name,
				isAdmin: false
			}
		});
	});
}

function taskColumn(page: Page, status: string): Locator {
	return page.locator(`[data-flow-board-column="${status}"]`);
}

function taskCard(page: Page, taskID: string): Locator {
	return page.locator(`[data-flow-board-card="${taskID}"]`);
}

function columnAppendTarget(page: Page, status: string): Locator {
	return page.locator(`[data-flow-board-drop-zone="${status}"]`);
}

function insertionIndicator(page: Page, status: string, beforeTaskID: string): Locator {
	return page.locator(`[data-flow-board-drop-indicator="${status}:${beforeTaskID}"]`);
}

async function visibleBoundingBox(locator: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
	await expect(locator).toBeVisible();
	const box = await locator.boundingBox();
	if (!box) throw new Error('locator has no visible bounding box');
	return box;
}

async function progressCardIDs(page: Page): Promise<string[]> {
	return taskColumn(page, '진행').locator('[data-flow-board-card]').evaluateAll((elements) =>
		elements.map((element) => element.getAttribute('data-flow-board-card') ?? '')
			.filter((taskID) => taskID.startsWith('26W23-'))
	);
}

function flowTaskBoardMovePayload(document: string | null): { taskID: string } {
	if (!document) return { taskID: '' };
	const parsed: unknown = JSON.parse(document);
	if (!isUnknownRecord(parsed)) return { taskID: '' };
	const taskID = parsed.taskID;
	return { taskID: typeof taskID === 'string' ? taskID : '' };
}

function isUnknownRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

async function dragToLocator(source: Locator, target: Locator): Promise<void> {
	const box = await target.boundingBox();
	if (!box) throw new Error('target locator has no bounding box');
	const dataTransfer = await source.evaluateHandle(() => new DataTransfer());
	await source.dispatchEvent('dragstart', { dataTransfer });
	await target.dispatchEvent('dragover', {
		clientY: box.y + Math.max(2, box.height / 2),
		dataTransfer
	});
	await target.dispatchEvent('drop', {
		clientY: box.y + Math.max(2, box.height / 2),
		dataTransfer
	});
	await source.dispatchEvent('dragend', { dataTransfer });
}

async function showUpperInsertionIndicator(page: Page, source: Locator, target: Locator): Promise<void> {
	const targetBox = await target.boundingBox();
	if (!targetBox) throw new Error('target locator has no bounding box');
	const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
	await source.dispatchEvent('dragstart', { dataTransfer });
	await target.dispatchEvent('dragover', {
		clientY: targetBox.y + targetBox.height * 0.25,
		dataTransfer
	});
}

async function expectInsertionSlotAbove(indicator: Locator, target: Locator): Promise<void> {
	const indicatorBox = await indicator.boundingBox();
	const targetBox = await target.boundingBox();
	if (!indicatorBox) throw new Error('insertion indicator has no bounding box');
	if (!targetBox) throw new Error('target locator has no bounding box');
	expect(indicatorBox.height).toBeGreaterThanOrEqual(14);
	expect(indicatorBox.y + indicatorBox.height).toBeLessThanOrEqual(targetBox.y);
}

async function dragToUpperHalf(source: Locator, target: Locator): Promise<void> {
	const box = await target.boundingBox();
	if (!box) throw new Error('target locator has no bounding box');
	const dataTransfer = await source.evaluateHandle(() => new DataTransfer());
	await source.dispatchEvent('dragstart', { dataTransfer });
	await target.dispatchEvent('dragover', {
		clientY: box.y + box.height * 0.25,
		dataTransfer
	});
	await target.dispatchEvent('drop', {
		clientY: box.y + box.height * 0.25,
		dataTransfer
	});
	await source.dispatchEvent('dragend', { dataTransfer });
}
