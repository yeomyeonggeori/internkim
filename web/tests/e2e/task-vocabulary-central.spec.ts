import { expect, test, type Locator } from '@playwright/test';
import { member1ID } from './central-test-utils';
import {
	companyTaskVocabulary,
	removeTasks,
	seedTasks,
	setCompanyTaskVocabulary,
	signInToTheTaskBoard,
	taskCard,
	type CentralTaskVocabulary
} from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

const businessName = '사업하나';
const typeName = '기능';
const repaintedBusinessColor = '#b91c1c';
const repaintedTypeColor = '#047857';

let seededTaskIDs: string[] = [];
let labelledTaskID = '';
let unlabelledTaskID = '';
let seededVocabulary: CentralTaskVocabulary = {};

function rgbOf(color: string): string {
	const red = Number.parseInt(color.slice(1, 3), 16);
	const green = Number.parseInt(color.slice(3, 5), 16);
	const blue = Number.parseInt(color.slice(5, 7), 16);
	return `rgb(${red}, ${green}, ${blue})`;
}

function businessBadgeOf(card: Locator): Locator {
	return card.locator('[data-slot="badge"]').filter({ hasText: businessName });
}

function typeBadgeOf(card: Locator, label: string): Locator {
	return card.locator('[data-slot="badge"]').filter({ hasText: label });
}

test.beforeAll(async () => {
	seededVocabulary = await companyTaskVocabulary();
	seededTaskIDs = await seedTasks([
		{
			title: 'E2E 어휘 색이 있는 업무',
			status: 'in_progress',
			participantIDs: [member1ID],
			business: businessName,
			type: typeName,
			size: 'M'
		},
		{
			title: 'E2E 어휘 색이 없는 업무',
			status: 'in_progress',
			participantIDs: [member1ID],
			business: null,
			type: null,
			size: 'M'
		}
	]);
	[labelledTaskID, unlabelledTaskID] = seededTaskIDs;
});

test.afterAll(async () => {
	await setCompanyTaskVocabulary(seededVocabulary);
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

test('a business badge is painted the colour the record holds for that business', async ({ page }) => {
	const storedColor = seededVocabulary.businesses?.find((business) => business.name === businessName)?.color;
	expect(storedColor).toBeTruthy();
	if (!storedColor) return;

	await signInToTheTaskBoard(page);
	const badge = businessBadgeOf(taskCard(page, labelledTaskID));
	await expect(badge).toBeVisible();
	expect(await badge.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(rgbOf(storedColor));
});

test('a colour repainted on the record repaints the badge', async ({ page }) => {
	await setCompanyTaskVocabulary({
		...seededVocabulary,
		businesses: (seededVocabulary.businesses ?? []).map((business) =>
			business.name === businessName ? { ...business, color: repaintedBusinessColor } : business
		),
		types: (seededVocabulary.types ?? []).map((type) =>
			type.name === typeName ? { ...type, color: repaintedTypeColor } : type
		)
	});

	await signInToTheTaskBoard(page);
	const card = taskCard(page, labelledTaskID);
	expect(
		await businessBadgeOf(card).evaluate((element) => getComputedStyle(element).backgroundColor)
	).toBe(rgbOf(repaintedBusinessColor));
	expect(
		await typeBadgeOf(card, typeName).evaluate((element) => getComputedStyle(element).color)
	).toBe(rgbOf(repaintedTypeColor));
});

test('a task the record gives no business and no type is labelled 기타 twice', async ({ page }) => {
	await signInToTheTaskBoard(page);

	const card = taskCard(page, unlabelledTaskID);
	await expect(card).toBeVisible();
	await expect(card.locator('[data-slot="badge"]').filter({ hasText: '기타' })).toHaveCount(2);
});
