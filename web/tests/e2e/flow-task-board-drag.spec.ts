import { expect, test } from '@playwright/test';
import {
	columnAppendTarget,
	dragToLocator,
	dragToUpperHalf,
	expectInsertionSlotAbove,
	flowDashboardTaskID,
	flowTaskBoardMovePayload,
	insertionIndicator,
	marketScanTaskID,
	openFlowBoard,
	progressCardIDs,
	requestedTaskID,
	scheduledTaskID,
	scrollFlowBoardToTop,
	scrollFlowPageBy,
	setFlowPageScrollTop,
	showUpperInsertionIndicator,
	taskCard,
	taskColumn,
	waitForFlowBoardHeightUpdate
} from './flow-task-helpers';

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
		await expect(taskColumn(page, '진행').locator('[data-flow-board-task-count]')).toHaveText('3');
		await expect(taskColumn(page, '진행').locator('[data-flow-board-footer-add-task="진행"]')).toHaveCSS('opacity', '0');
		await taskColumn(page, '진행').hover();
		await expect(taskColumn(page, '진행').locator('[data-flow-board-footer-add-task="진행"]')).toHaveCSS('opacity', '1');
		const flowDashboardGoal = '업무 진행도 화면에서 상태와 거리 흐름을 빠르게 본다.';
		await expect(taskColumn(page, '진행').getByText(flowDashboardGoal)).toHaveCount(0);
		await taskCard(page, flowDashboardTaskID).click();
		await expect(page.getByPlaceholder('완료 기준')).toHaveValue(flowDashboardGoal);
		await page.keyboard.press('Escape');
		await expect(page.getByPlaceholder('완료 기준')).toHaveCount(0);
		await expect(page.getByRole('button', { name: /필터/ })).toBeVisible();
		await expect(page.getByPlaceholder('내용, 목표, 참여자 검색')).toHaveCount(0);

		await page.getByRole('button', { name: /필터/ }).click();
		await expect(page.getByPlaceholder('내용, 목표, 참여자 검색')).toBeVisible();
		await expect(page.locator('[data-flow-filter-panel]').getByText('김철수')).toBeVisible();
	});

	test('preserves task filters and view mode when returning from another Flow tab', async ({ page }) => {
		await page.goto('/flow/');

		await page.getByRole('button', { name: /필터/ }).click();
		await page.locator('[data-flow-filter-panel]').getByRole('button', { name: '전체 참여자', exact: true }).click();
		await page.keyboard.press('Escape');
		await page.getByRole('tab', { name: '목록', exact: true }).click();
		await expect(page.getByRole('tab', { name: '목록', exact: true })).toHaveAttribute('aria-selected', 'true');

		await page.getByRole('button', { name: '보고', exact: true }).click();
		await expect(page.getByRole('heading', { name: '개인 상세 점수', exact: true })).toHaveCount(0);
		await page.getByRole('button', { name: '업무', exact: true }).click();

		await expect(page.getByRole('heading', { name: '개인 상세 점수', exact: true })).toBeVisible();
		await expect(page.getByRole('tab', { name: '목록', exact: true })).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByRole('button', { name: '필터', exact: true })).toBeVisible();
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

	test('keeps dragged card surface visible without hover border accent', async ({ page }) => {
		await page.goto('/flow/');
		const card = page.locator('[data-flow-board-card]').first();
		await expect(card).toBeVisible();

		const restingBorderColor = await card.evaluate((element) => getComputedStyle(element).borderColor);
		await card.hover();
		const hoverStyle = await card.evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				backgroundColor: style.backgroundColor,
				borderColor: style.borderColor,
				outlineStyle: style.outlineStyle
			};
		});
		expect(hoverStyle.borderColor).toBe(restingBorderColor);
		expect(hoverStyle.outlineStyle).toBe('none');

		const dataTransfer = await card.evaluateHandle(() => new DataTransfer());
		await card.dispatchEvent('dragstart', { dataTransfer });
		await expect(card).toHaveAttribute('data-flow-board-dragging', 'true');
		const dragBackgroundColor = await card.evaluate((element) => getComputedStyle(element).backgroundColor);
		expect(dragBackgroundColor).not.toBe('rgba(0, 0, 0, 0)');
		await card.dispatchEvent('dragend', { dataTransfer });
		await expect(card).toHaveAttribute('data-flow-board-dragging', 'false');

		const dragImageOffset = await card.evaluate((element) => {
			const dataTransfer = new DataTransfer();
			let offset = { horizontal: 0, vertical: 0 };
			dataTransfer.setDragImage = (_image: Element, horizontal: number, vertical: number) => {
				offset = { horizontal, vertical };
			};
			const bounds = element.getBoundingClientRect();
			element.dispatchEvent(new DragEvent('dragstart', {
				bubbles: true,
				cancelable: true,
				clientX: bounds.left + bounds.width * 0.6,
				clientY: bounds.top + bounds.height * 0.45,
				dataTransfer
			}));
			element.dispatchEvent(new DragEvent('dragend', { bubbles: true, dataTransfer }));
			return offset;
		});
		const cardBox = await card.boundingBox();
		if (!cardBox) throw new Error('card has no bounding box');
		expect(dragImageOffset.horizontal).toBeGreaterThan(cardBox.width * 0.55);
		expect(dragImageOffset.horizontal).toBeLessThan(cardBox.width * 0.65);
		expect(dragImageOffset.vertical).toBeGreaterThan(cardBox.height * 0.4);
		expect(dragImageOffset.vertical).toBeLessThan(cardBox.height * 0.5);
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
		await expect.poll(async () => {
			return page.locator('[data-flow-board-scroll]').evaluate((element) => getComputedStyle(element).scrollbarWidth);
		}).toBe('none');
	});

	test('lets the board scroller span from the sidebar edge to the viewport edge', async ({ page }) => {
		await page.setViewportSize({ width: 1002, height: 520 });
		await page.goto('/flow/');
		const board = page.locator('[data-flow-board-scroll]');
		await expect(board).toBeVisible();
		await board.scrollIntoViewIfNeeded();

		await expectBoardScrollerToFillMainViewport(page);
	});

	test('resizes board columns with the viewport height', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);
		const mediumHeight = await taskColumn(page, '진행').evaluate((element) => element.getBoundingClientRect().height);

		await page.setViewportSize({ width: 1440, height: 1200 });
		await expect.poll(async () => {
			return taskColumn(page, '진행').evaluate((element) => element.getBoundingClientRect().height);
		}).toBeGreaterThan(mediumHeight + 80);
	});

	test('resizes board columns after scrolling the board into view', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await openFlowBoard(page);
		await setFlowPageScrollTop(page, 0);
		const topHeight = await taskColumn(page, '진행').evaluate((element) => element.getBoundingClientRect().height);

		await scrollFlowBoardToTop(page);

		await expect.poll(async () => {
			return taskColumn(page, '진행').evaluate((element) => element.getBoundingClientRect().height);
		}).toBeGreaterThan(topHeight + 120);

		await scrollFlowPageBy(page, 2000);
		await waitForFlowBoardHeightUpdate(page);
		const cappedHeight = await taskColumn(page, '진행').evaluate((element) => element.getBoundingClientRect().height);

		await scrollFlowPageBy(page, 2000);
		await waitForFlowBoardHeightUpdate(page);

		await expect.poll(async () => {
			const currentHeight = await taskColumn(page, '진행').evaluate((element) => element.getBoundingClientRect().height);
			return currentHeight <= cappedHeight + 1;
		}).toBe(true);
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
});

async function expectBoardScrollerToFillMainViewport(page: import('@playwright/test').Page): Promise<void> {
	const measurement = await page.evaluate(() => {
		const mainElement = document.querySelector('main');
		const scrollElement = document.querySelector('[data-flow-board-scroll]');
		const firstColumnElement = document.querySelector('[data-flow-board-column="요청"]');
		const fourthColumnElement = document.querySelector('[data-flow-board-column="완료"]');
		if (!mainElement) throw new Error('main element was not found');
		if (!scrollElement) throw new Error('flow board scroll element was not found');
		if (!firstColumnElement) throw new Error('requested flow board column was not found');
		if (!fourthColumnElement) throw new Error('completed flow board column was not found');
		const mainBounds = mainElement.getBoundingClientRect();
		const scrollBounds = scrollElement.getBoundingClientRect();
		const firstColumnBounds = firstColumnElement.getBoundingClientRect();
		const fourthColumnBounds = fourthColumnElement.getBoundingClientRect();
		const fourthColumnVisibleWidth = Math.min(scrollBounds.right, fourthColumnBounds.right) - Math.max(scrollBounds.left, fourthColumnBounds.left);
		return {
			scrollLeft: scrollBounds.left,
			scrollRight: scrollBounds.right,
			mainLeft: mainBounds.left,
			viewportRight: window.innerWidth,
			firstColumnLeftGap: firstColumnBounds.left - scrollBounds.left,
			fourthColumnVisibleRatio: fourthColumnVisibleWidth / fourthColumnBounds.width,
			scrollWidth: scrollElement.scrollWidth,
			clientWidth: scrollElement.clientWidth
		};
	});
	expect(measurement.scrollLeft).toBeCloseTo(measurement.mainLeft, 0);
	expect(measurement.scrollRight).toBeCloseTo(measurement.viewportRight, 0);
	expect(measurement.firstColumnLeftGap).toBeGreaterThanOrEqual(30);
	expect(measurement.firstColumnLeftGap).toBeLessThanOrEqual(34);
	expect(measurement.fourthColumnVisibleRatio).toBeGreaterThan(0.4);
	expect(measurement.fourthColumnVisibleRatio).toBeLessThan(0.65);
	expect(measurement.scrollWidth).toBeGreaterThan(measurement.clientWidth);
}
