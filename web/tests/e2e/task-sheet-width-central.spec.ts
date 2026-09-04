import { expect, test, type Page } from '@playwright/test';
import { member1ID } from './central-test-utils';
import { openTaskCard, removeTasks, seedTasks, signInToTheTaskBoard, taskSheet } from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const mobileViewport = { width: 390, height: 844 };
const desktopViewport = { width: 1280, height: 900 };

let seededTaskIDs: string[] = [];

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

async function openTaskSheet(page: Page) {
	const [taskID] = await seedTasks([
		{ title: 'E2E 시트 폭 확인 업무', status: 'planned', participantIDs: [member1ID] }
	]);
	seededTaskIDs.push(taskID);
	await signInToTheTaskBoard(page);
	await openTaskCard(page, taskID);
	return taskSheet(page);
}

test('a right sheet asking for w-full sm:max-w-xl renders at the full mobile viewport width', async ({ page }) => {
	await page.setViewportSize(mobileViewport);
	const sheet = await openTaskSheet(page);

	const width = await sheet.evaluate((element) => element.getBoundingClientRect().width);
	expect(width).toBe(mobileViewport.width);
});

test('a right sheet asking for w-full sm:max-w-xl renders at sm:max-w-xl on desktop', async ({ page }) => {
	await page.setViewportSize(desktopViewport);
	const sheet = await openTaskSheet(page);

	const width = await sheet.evaluate((element) => element.getBoundingClientRect().width);
	expect(width).toBeCloseTo(576, 0);
});
