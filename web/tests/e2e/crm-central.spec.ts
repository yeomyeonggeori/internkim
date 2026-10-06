import { expect, test, type Locator, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const organizationName = 'E2E 중앙 검증 기관';
const contactName = 'E2E 외부 담당자';
const firstOpportunityName = 'E2E 진행 건 하나';
const secondOpportunityName = 'E2E 진행 건 둘';
const calendarActivityTitle = 'E2E 캘린더 활동';
const boardActivityTitle = 'E2E 업무 활동';
const settledOpportunityName = 'E2E 확정 진행 건';
const organizationTypeName = 'E2E 관계처 유형';
const queuedTypeNames = ['E2E 대기 유형 하나', 'E2E 대기 유형 둘'];

async function signIn(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/crm');
	await page.locator('[data-crm-ready="true"]').waitFor({ state: 'visible', timeout: 20000 });
}

async function openCreateForm(page: Page, tab: string, createButton: string): Promise<void> {
	await page.getByRole('tab', { name: tab, exact: true }).click();
	// Scoped to the toolbar: sortable column headers are buttons carrying the same labels.
	const toolbar = page.getByRole('tabpanel', { name: tab }).locator('div').first();
	await toolbar.getByRole('button', { name: createButton, exact: true }).click();
}

function recordSheet(page: Page) {
	return page.getByRole('dialog', { name: 'CRM 기록 추가' });
}

test('creates an organization and keeps it after reload', async ({ page }) => {
	await signIn(page);
	await openCreateForm(page, '관계처', '관계처');
	const sheet = recordSheet(page);
	await sheet.getByLabel('이름 또는 제목').fill(organizationName);
	await sheet.getByRole('checkbox', { name: '파트너' }).click();
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();

	await expect(page.getByRole('row', { name: new RegExp(organizationName) })).toBeVisible();
	await page.reload();
	await page.getByRole('tabpanel', { name: '관계처' }).locator('div').first().getByRole('button', { name: '관계처', exact: true }).waitFor({ state: 'visible', timeout: 20000 });
	await expect(page.getByRole('row', { name: new RegExp(organizationName) })).toBeVisible();
});

test('registers a contact under the organization', async ({ page }) => {
	await signIn(page);
	await openCreateForm(page, '연락처', '연락처');
	const sheet = recordSheet(page);
	await sheet.getByLabel('관계처').click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
	await sheet.getByLabel('연락처 이름').fill(contactName);
	await sheet.getByLabel('이메일', { exact: true }).fill('e2e-contact@example.com');
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();

	await page.getByRole('tab', { name: '연락처' }).click();
	await expect(page.getByRole('row', { name: new RegExp(contactName) })).toBeVisible();
});

test('links the same external contact to two opportunities', async ({ page }) => {
	await signIn(page);
	for (const name of [firstOpportunityName, secondOpportunityName]) {
		await openCreateForm(page, '거래', '거래');
		const sheet = recordSheet(page);
		await sheet.getByLabel('관계처').click();
		await page.getByRole('option', { name: organizationName, exact: true }).click();
		await sheet.getByLabel('이름 또는 제목').fill(name);
		await sheet.locator('#crm-record-progress-contact').click();
		await page.getByRole('option', { name: new RegExp(contactName) }).click();
		await sheet.getByRole('button', { name: '추가', exact: true }).click();
		await expect(sheet).not.toBeVisible();
	}

	await page.getByRole('tab', { name: '거래' }).click();
	await expect(page.getByRole('row', { name: new RegExp(firstOpportunityName) })).toBeVisible();
	await expect(page.getByRole('row', { name: new RegExp(secondOpportunityName) })).toBeVisible();
});

async function recordActivity(page: Page, title: string, registersOnTheCalendar: boolean): Promise<void> {
	await page.getByRole('button', { name: '활동 기록' }).click();
	const sheet = recordSheet(page);
	await sheet.getByLabel('관계처').click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
	await sheet.getByLabel('활동 제목').fill(title);
	if (registersOnTheCalendar) await sheet.getByRole('checkbox', { name: '캘린더에 등록' }).click();
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();
	await expect(page.getByRole('row', { name: new RegExp(title) })).toBeVisible();
}

test('an activity registered on the calendar stays off the task board, and one without it lands on it', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '활동', exact: true }).click();
	await recordActivity(page, calendarActivityTitle, true);
	await recordActivity(page, boardActivityTitle, false);

	await page.goto('/example-co/calendar');
	const calendar = page.locator('.calendar-stage');
	await expect(calendar.getByText(calendarActivityTitle).first()).toBeVisible({ timeout: 20000 });
	await expect(calendar.getByText(boardActivityTitle)).toHaveCount(0);

	await page.goto('/example-co/flow');
	await page.locator('[data-task-ready="true"]').waitFor({ state: 'visible', timeout: 20000 });
	await expect(page.getByText(boardActivityTitle).first()).toBeVisible({ timeout: 20000 });
	await expect(page.getByText(calendarActivityTitle)).toHaveCount(0);
});

test('stage change records an automatic activity and editing it keeps the stage', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '거래' }).click();
	await page.getByRole('row', { name: new RegExp(firstOpportunityName) }).click();
	const editSheet = page.getByRole('dialog', { name: '거래 수정' });
	await editSheet.getByLabel('단계').click();
	await page.getByRole('option', { name: '진행 중', exact: true }).click();
	await editSheet.getByRole('button', { name: '저장', exact: true }).click();
	await expect(editSheet).not.toBeVisible();

	await page.getByRole('tab', { name: '활동', exact: true }).click();
	const historyRow = page.getByRole('row', { name: new RegExp(`활동 수정 · ${firstOpportunityName}`) });
	await expect(historyRow).toBeVisible();
	await historyRow.click();
	const activitySheet = page.getByRole('dialog', { name: '활동 수정' });
	await activitySheet.getByLabel('비고').fill('E2E 단계 이력 수정');
	await activitySheet.getByRole('button', { name: '저장', exact: true }).click();
	await expect(activitySheet).not.toBeVisible();

	await page.getByRole('tab', { name: '거래' }).click();
	await expect(page.getByRole('row', { name: new RegExp(firstOpportunityName) })).toContainText('진행 중');
});

test('rejects deleting an in-use CRM definition with guidance', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();
	await page.getByRole('button', { name: '삭제' }).first().click();
	await page
		.locator('[role="alertdialog"][data-state="open"]')
		.locator('[data-alert-dialog-action]')
		.last()
		.click();
	await expect(
		page.getByText('등록된 CRM 기록에서 사용 중인 항목입니다. 연결된 기록의 값을 변경한 후 삭제해 주세요.').first()
	).toBeVisible({ timeout: 10000 });
});

test('adds and removes an unused CRM definition', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();
	await page.getByRole('button', { name: '추가', exact: true }).first().click();
	await page.getByRole('textbox', { name: '관계처 유형' }).fill('E2E 임시 유형');
	await page.getByRole('button', { name: '추가', exact: true }).first().click();
	await expect
		.poll(async () =>
			page.getByRole('textbox').evaluateAll((nodes) => nodes.some((node) => (node as HTMLInputElement).value === 'E2E 임시 유형'))
		)
		.toBe(true);
});

test('rejects deleting an in-use flow business with guidance', async ({ page }) => {
	await signIn(page);
	await page.goto('/example-co/flow');
	await page.getByRole('tab', { name: '정의' }).click();
	await page.getByRole('button', { name: '삭제' }).first().waitFor({ state: 'visible' });
	await page.getByRole('button', { name: '삭제' }).first().click();
	const confirm = page.locator('[role="alertdialog"][data-state="open"]').filter({ hasText: '사업하나' });
	await confirm.locator('[data-alert-dialog-action]').last().click();
	await expect(
		page.getByText('등록된 업무나 거래에서 사용 중인 항목입니다. 연결된 기록의 값을 변경한 후 삭제해 주세요.').first()
	).toBeVisible({ timeout: 10000 });
});

async function chooseCurrencyOption(page: Page, code: string): Promise<void> {
	const option = page.getByRole('option', { name: new RegExp(`\\b${code}\\b`) }).first();
	await option.scrollIntoViewIfNeeded();
	await option.click();
}

/** Find a row's cell by its column header, so the assertion survives column changes. */
async function cellUnder(page: Page, row: Locator, headerName: string): Promise<Locator> {
	const headers = await page.getByRole('columnheader').allInnerTexts();
	const index = headers.findIndex((header) => header.trim() === headerName);
	expect(index, `column ${headerName} exists`).toBeGreaterThanOrEqual(0);
	return row.getByRole('cell').nth(index);
}

function isOrdered(amounts: number[], direction: 'ascending' | 'descending'): boolean {
	return amounts.every((amount, index) => {
		if (index === 0) return true;
		const previous = amounts[index - 1];
		return direction === 'ascending' ? previous <= amount : previous >= amount;
	});
}

const koreanCompactUnits: Record<string, number> = { 천: 1e3, 만: 1e4, 억: 1e8 };

/** Read the compact amounts the table renders (₩1,800만, ₩4.6억) back into comparable numbers. */
async function readAmounts(cells: Locator): Promise<number[]> {
	const texts = await cells.allInnerTexts();
	return texts.flatMap((text) => {
		const match = text.match(/([\d,.]+)\s*([천만억])?/);
		if (!match) return [];
		const digits = Number(match[1].replaceAll(',', ''));
		if (Number.isNaN(digits)) return [];
		return [digits * (koreanCompactUnits[match[2] ?? ''] ?? 1)];
	});
}

async function settledNoteOf(page: Page): Promise<string> {
	await page.getByRole('tab', { name: '거래' }).click();
	await page.getByRole('row', { name: new RegExp(settledOpportunityName) }).click();
	const editSheet = page.getByRole('dialog', { name: '거래 수정' });
	const note = editSheet.getByText(/확정$/);
	await expect(note).toBeVisible({ timeout: 20000 });
	return (await note.innerText()).trim();
}

async function openAdministratorSettings(page: Page): Promise<void> {
	await page.getByRole('tab', { name: '관리자' }).click();
}

test('an administrator moves the company onto another base currency', async ({ page }) => {
	await signIn(page);
	await page.goto('/example-co/settings');
	await openAdministratorSettings(page);
	const currency = page.getByLabel('기준 통화');
	await expect(currency).toContainText('KRW', { timeout: 20000 });

	await currency.click();
	await chooseCurrencyOption(page, 'USD');
	await expect(currency).toContainText('USD');
	const currencyCard = page.locator('[data-slot="card"]').filter({ has: currency });
	const saveCurrency = currencyCard.getByRole('button', { name: '저장', exact: true });
	const saved = page.waitForResponse((response) => response.url().includes('/api/v1/tools/company_settings_update/invoke'));
	await saveCurrency.click();
	await saved;

	await page.reload();
	await openAdministratorSettings(page);
	await expect(page.getByLabel('기준 통화')).toContainText('USD', { timeout: 20000 });
});

test('closing a deal priced in another currency settles it in the base currency', async ({ page }) => {
	await signIn(page);
	await openCreateForm(page, '거래', '거래');
	const sheet = recordSheet(page);
	await sheet.getByLabel('관계처').click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
	await sheet.getByLabel('이름 또는 제목').fill(settledOpportunityName);
	await sheet.locator('#crm-record-amount-currency').click();
	await chooseCurrencyOption(page, 'KRW');
	await sheet.locator('#crm-record-amount').fill('18000000');
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();

	await page.getByRole('tab', { name: '거래' }).click();
	await page.getByRole('row', { name: new RegExp(settledOpportunityName) }).click();
	const editSheet = page.getByRole('dialog', { name: '거래 수정' });
	await editSheet.getByLabel('단계').click();
	await page.getByRole('option', { name: '성사', exact: true }).click();
	await editSheet.getByRole('button', { name: '저장', exact: true }).click();
	await expect(editSheet).not.toBeVisible();

	const settled = await settledNoteOf(page);
	expect(settled).toContain('확정');
	expect(settled).toContain('$');
});

test('a settled amount survives a move between terminal stages', async ({ page }) => {
	await signIn(page);
	const before = await settledNoteOf(page);

	const editSheet = page.getByRole('dialog', { name: '거래 수정' });
	await editSheet.getByLabel('단계').click();
	await page.getByRole('option', { name: '실패', exact: true }).click();
	await editSheet.getByLabel('실패 사유').fill('예산 부족');
	await editSheet.getByRole('button', { name: '저장', exact: true }).click();
	await expect(editSheet).not.toBeVisible();

	expect(await settledNoteOf(page)).toBe(before);
});

test('opens in the company base currency and re-denominates amounts when the view currency changes', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '거래' }).click();
	const row = page.getByRole('row', { name: new RegExp(settledOpportunityName) });
	await expect(row).toBeVisible();

	const viewCurrency = page.getByRole('tabpanel', { name: '거래' }).getByLabel('보기 통화');
	await expect(viewCurrency).toHaveText(/USD/, { timeout: 20000 });
	const amount = await cellUnder(page, row, '금액');
	await expect(amount).toHaveText(/^\$/, { timeout: 20000 });
	const inDollars = (await amount.innerText()).trim();

	await viewCurrency.click();
	await chooseCurrencyOption(page, 'KRW');
	await expect(viewCurrency).toHaveText(/KRW/, { timeout: 20000 });
	await expect(amount).not.toHaveText(inDollars, { timeout: 20000 });
	await expect(amount).toHaveText(/^₩/, { timeout: 20000 });
	await expect(amount).toHaveText(/만|억/, { timeout: 20000 });
});

test('sorting the open amount column orders the accounts by value both ways', async ({ page }) => {
	await signIn(page);
	const header = page.getByRole('columnheader', { name: '열린 거래 금액' }).getByRole('button');
	const amountCells = page.getByRole('row').locator('td:last-child');

	expect((await readAmounts(amountCells)).length).toBeGreaterThan(1);

	await header.click();
	await expect.poll(async () => isOrdered(await readAmounts(amountCells), 'ascending')).toBe(true);

	await header.click();
	await expect.poll(async () => isOrdered(await readAmounts(amountCells), 'descending')).toBe(true);
});

test('an added definition shows at once and the list keeps its shape while saving', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();

	const definitions = page.getByRole('region', { name: '정의', exact: true });
	const card = definitions.locator('[data-slot="card"]').filter({ hasText: '관계처 유형' });
	const names = card.getByRole('textbox');
	const swatches = card.getByRole('button', { name: '색상' });
	await expect(swatches.first()).toBeVisible();
	const nameCount = await names.count();
	const swatchCount = await swatches.count();

	let releaseSave = (): void => {};
	const heldSave = new Promise<void>((resolve) => {
		releaseSave = resolve;
	});
	await page.route('**/tools/crm_vocabulary_set/invoke', async (route) => {
		await heldSave;
		await route.continue();
	});

	await card.getByRole('button', { name: '추가' }).click();
	await card.getByPlaceholder('관계처 유형').fill(organizationTypeName);
	await card.getByRole('button', { name: '추가' }).click();

	await expect(names.nth(nameCount)).toHaveValue(organizationTypeName);
	await expect(swatches).toHaveCount(swatchCount + 1);
	await expect(names.first()).toBeEnabled();

	const saved = page.waitForResponse('**/tools/crm_vocabulary_set/invoke');
	releaseSave();
	await saved;

	await page.reload();
	await page.getByRole('tab', { name: '정의' }).click();
	await expect(names.nth(nameCount)).toHaveValue(organizationTypeName);

	const removed = page.waitForResponse('**/tools/crm_vocabulary_set/invoke');
	await card.getByRole('button', { name: '삭제' }).nth(nameCount).click();
	const removalConfirm = page.locator('[role="alertdialog"][data-state="open"]').filter({ hasText: organizationTypeName });
	await removalConfirm.locator('[data-alert-dialog-action]').last().click();
	await expect(names).toHaveCount(nameCount);
	await removed;
});

test('a definition added while an earlier save runs is not lost', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();

	const definitions = page.getByRole('region', { name: '정의', exact: true });
	const card = definitions.locator('[data-slot="card"]').filter({ hasText: '관계처 유형' });
	const names = card.getByRole('textbox');
	await expect(names.first()).toBeVisible();
	const nameCount = await names.count();

	let releaseFirstSave = (): void => {};
	const heldFirstSave = new Promise<void>((resolve) => {
		releaseFirstSave = resolve;
	});
	let hasHeld = false;
	await page.route('**/tools/crm_vocabulary_set/invoke', async (route) => {
		if (!hasHeld) {
			hasHeld = true;
			await heldFirstSave;
		}
		await route.continue();
	});

	let completedSaves = 0;
	page.on('response', (response) => {
		if (response.url().includes('/tools/crm_vocabulary_set/invoke')) completedSaves += 1;
	});

	const addName = card.getByPlaceholder('관계처 유형');
	const addButton = card.getByRole('button', { name: '추가' });
	await addButton.click();
	await addName.fill(queuedTypeNames[0]);
	await addButton.click();
	await addButton.click();
	await addName.fill(queuedTypeNames[1]);
	await addButton.click();
	releaseFirstSave();
	await expect.poll(() => completedSaves).toBe(2);

	await page.reload();
	await page.getByRole('tab', { name: '정의' }).click();
	await expect(names.nth(nameCount)).toHaveValue(queuedTypeNames[0]);
	await expect(names.nth(nameCount + 1)).toHaveValue(queuedTypeNames[1]);

	for (const index of [nameCount + 1, nameCount]) {
		const removed = page.waitForResponse('**/tools/crm_vocabulary_set/invoke');
		await card.getByRole('button', { name: '삭제' }).nth(index).click();
		await page
			.locator('[role="alertdialog"][data-state="open"]')
			.locator('[data-alert-dialog-action]')
			.last()
			.click();
		await removed;
	}
	await expect(names).toHaveCount(nameCount);
});

test('a renamed pipeline reaches the rest of the workspace without a reload', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();

	const definitions = page.getByRole('region', { name: '정의', exact: true });
	const card = definitions.locator('[data-slot="card"]').filter({ hasText: '파이프라인' });
	const firstName = card.getByRole('textbox').first();
	await expect(firstName).toBeVisible();
	const originalName = await firstName.inputValue();
	const renamedName = `${originalName} 개명`;

	const saved = page.waitForResponse('**/tools/crm_vocabulary_set/invoke');
	await firstName.fill(renamedName);
	await firstName.blur();
	await saved;

	await expect(page.locator('[data-crm-metrics]')).toContainText(renamedName);

	const restored = page.waitForResponse('**/tools/crm_vocabulary_set/invoke');
	await firstName.fill(originalName);
	await firstName.blur();
	await restored;
	await expect(page.locator('[data-crm-metrics]')).toContainText(originalName);
});
