import { expect, type Locator, type Page } from '@playwright/test';

export const requestedTaskID = '26W23-attendance-policy';
export const scheduledTaskID = '26W23-mail-triage';
export const flowDashboardTaskID = '26W23-flow-dashboard';
export const marketScanTaskID = '26W23-market-scan';

export async function openFlowBoard(page: Page): Promise<void> {
	await page.goto('/flow/');
	await expect(page.getByRole('button', { name: '업무', exact: true })).toBeVisible();
	await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
	await page.getByRole('button', { name: /필터/ }).click();
	await page.locator('[data-flow-filter-panel]').getByRole('button', { name: '전체 참여자', exact: true }).click();
	await page.keyboard.press('Escape');
	await expect(taskColumn(page, '진행')).toBeVisible();
}

export async function useMemberFlowSession(page: Page, email: string, name: string): Promise<void> {
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

export function taskColumn(page: Page, status: string): Locator {
	return page.locator(`[data-flow-board-column="${status}"]`);
}

export function taskCard(page: Page, taskID: string): Locator {
	return page.locator(`[data-flow-board-card="${taskID}"]`);
}

export function columnAppendTarget(page: Page, status: string): Locator {
	return page.locator(`[data-flow-board-drop-zone="${status}"]`);
}

export function insertionIndicator(page: Page, status: string, beforeTaskID: string): Locator {
	return page.locator(`[data-flow-board-drop-indicator="${status}:${beforeTaskID}"]`);
}

export async function visibleBoundingBox(locator: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
	await expect(locator).toBeVisible();
	const box = await locator.boundingBox();
	if (!box) throw new Error('locator has no visible bounding box');
	return box;
}

export async function progressCardIDs(page: Page): Promise<string[]> {
	return taskColumn(page, '진행').locator('[data-flow-board-card]').evaluateAll((elements) =>
		elements.map((element) => element.getAttribute('data-flow-board-card') ?? '')
			.filter((taskID) => taskID.startsWith('26W23-'))
	);
}

export function flowTaskBoardMovePayload(document: string | null): { taskID: string } {
	if (!document) return { taskID: '' };
	const parsed: unknown = JSON.parse(document);
	if (!isUnknownRecord(parsed)) return { taskID: '' };
	const taskID = parsed.taskID;
	return { taskID: typeof taskID === 'string' ? taskID : '' };
}

export async function dragToLocator(source: Locator, target: Locator): Promise<void> {
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

export async function showUpperInsertionIndicator(page: Page, source: Locator, target: Locator): Promise<void> {
	const targetBox = await target.boundingBox();
	if (!targetBox) throw new Error('target locator has no bounding box');
	const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
	await source.dispatchEvent('dragstart', { dataTransfer });
	await target.dispatchEvent('dragover', {
		clientY: targetBox.y + targetBox.height * 0.25,
		dataTransfer
	});
}

export async function expectInsertionSlotAbove(indicator: Locator, target: Locator): Promise<void> {
	const indicatorBox = await indicator.boundingBox();
	const targetBox = await target.boundingBox();
	if (!indicatorBox) throw new Error('insertion indicator has no bounding box');
	if (!targetBox) throw new Error('target locator has no bounding box');
	expect(indicatorBox.height).toBeGreaterThanOrEqual(14);
	expect(indicatorBox.y + indicatorBox.height).toBeLessThanOrEqual(targetBox.y);
}

export async function dragToUpperHalf(source: Locator, target: Locator): Promise<void> {
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

export async function setFlowPageScrollTop(page: Page, scrollTop: number): Promise<void> {
	await updateFlowPageScroll(page, { type: 'set', value: scrollTop });
}

export async function scrollFlowBoardToTop(page: Page): Promise<void> {
	await updateFlowPageScroll(page, { type: 'alignBoardTop', value: 0 });
}

export async function scrollFlowPageBy(page: Page, offset: number): Promise<void> {
	await updateFlowPageScroll(page, { type: 'offset', value: offset });
}

export async function waitForFlowBoardHeightUpdate(page: Page): Promise<void> {
	await page.evaluate(() => new Promise<void>((resolve) => {
		requestAnimationFrame(() => {
			requestAnimationFrame(() => resolve());
		});
	}));
}

export function isUnknownRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

type FlowPageScrollInstruction = {
	type: 'alignBoardTop' | 'offset' | 'set';
	value: number;
};

async function updateFlowPageScroll(page: Page, instruction: FlowPageScrollInstruction): Promise<void> {
	await page.locator('[data-flow-board-scroll]').evaluate((element, nextInstruction) => {
		let scrollParent = element.parentElement;
		while (scrollParent && !['auto', 'scroll', 'overlay'].includes(getComputedStyle(scrollParent).overflowY)) {
			scrollParent = scrollParent.parentElement;
		}
		if (!scrollParent) {
			if (nextInstruction.type === 'alignBoardTop') {
				element.scrollIntoView();
			} else if (nextInstruction.type === 'offset') {
				window.scrollBy({ top: nextInstruction.value });
			} else {
				window.scrollTo({ top: nextInstruction.value });
			}
			window.dispatchEvent(new Event('scroll'));
			return;
		}
		if (nextInstruction.type === 'alignBoardTop') {
			const scrollParentBounds = scrollParent.getBoundingClientRect();
			const elementBounds = element.getBoundingClientRect();
			scrollParent.scrollTop += elementBounds.top - scrollParentBounds.top;
		} else if (nextInstruction.type === 'offset') {
			scrollParent.scrollTop += nextInstruction.value;
		} else {
			scrollParent.scrollTop = nextInstruction.value;
		}
		scrollParent.dispatchEvent(new Event('scroll'));
	}, instruction);
}
