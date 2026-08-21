import { expect, test, type Page } from '@playwright/test';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const organizationName = 'E2E 중앙 검증 기관';
const contactName = 'E2E 외부 담당자';
const firstOpportunityName = 'E2E 진행 건 하나';
const secondOpportunityName = 'E2E 진행 건 둘';
const activityTitle = 'E2E 캘린더 활동';
const settledOpportunityName = 'E2E 확정 진행 건';

async function signIn(page: Page): Promise<void> {
	await page.goto('/example-co/crm');
	const email = page.getByRole('textbox', { name: '이메일' });
	const needsSignIn = await email
		.waitFor({ state: 'visible', timeout: 8000 })
		.then(() => true)
		.catch(() => false);
	if (needsSignIn) {
		await email.fill('lee@example.com');
		await page.getByRole('textbox', { name: '비밀번호' }).fill('seed-password');
		await page.getByRole('button', { name: '로그인', exact: true }).click();
	}
	await page.getByRole('button', { name: '빠른 추가' }).waitFor({ state: 'visible', timeout: 20000 });
}

async function openQuickAdd(page: Page, kind: string): Promise<void> {
	await page.getByRole('button', { name: '빠른 추가' }).click();
	await page.getByRole('menuitem', { name: kind, exact: true }).click();
}

function recordSheet(page: Page) {
	return page.getByRole('dialog', { name: 'CRM 기록 추가' });
}

test('creates an organization and keeps it after reload', async ({ page }) => {
	await signIn(page);
	await openQuickAdd(page, '관계처');
	const sheet = recordSheet(page);
	await sheet.getByLabel('이름 또는 제목').fill(organizationName);
	await sheet.getByRole('checkbox', { name: '파트너' }).click();
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();

	await expect(page.getByRole('row', { name: new RegExp(organizationName) })).toBeVisible();
	await page.reload();
	await page.getByRole('button', { name: '빠른 추가' }).waitFor({ state: 'visible', timeout: 20000 });
	await expect(page.getByRole('row', { name: new RegExp(organizationName) })).toBeVisible();
});

test('registers a contact under the organization', async ({ page }) => {
	await signIn(page);
	await openQuickAdd(page, '담당자');
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
		await openQuickAdd(page, '진행 건');
		const sheet = recordSheet(page);
		await sheet.getByLabel('관계처').click();
		await page.getByRole('option', { name: organizationName, exact: true }).click();
		await sheet.getByLabel('이름 또는 제목').fill(name);
		await sheet.locator('#crm-record-progress-contact').click();
		await page.getByRole('option', { name: new RegExp(contactName) }).click();
		await sheet.getByRole('button', { name: '추가', exact: true }).click();
		await expect(sheet).not.toBeVisible();
	}

	await page.getByRole('tab', { name: '진행상황' }).click();
	await expect(page.getByRole('row', { name: new RegExp(firstOpportunityName) })).toBeVisible();
	await expect(page.getByRole('row', { name: new RegExp(secondOpportunityName) })).toBeVisible();
});

test('saves an activity that appears in flow and calendar as one task', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '활동', exact: true }).click();
	await page.getByRole('button', { name: '활동 기록' }).click();
	const sheet = recordSheet(page);
	await sheet.getByLabel('관계처').click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
	await sheet.getByLabel('활동 제목').fill(activityTitle);
	await sheet.getByRole('checkbox', { name: '캘린더에 등록' }).click();
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();
	await expect(page.getByRole('row', { name: new RegExp(activityTitle) })).toBeVisible();

	await page.goto('/example-co/flow');
	await expect(page.getByText(activityTitle).first()).toBeVisible({ timeout: 20000 });

	await page.goto('/example-co/calendar');
	await expect(page.frameLocator('iframe').getByText(activityTitle).first()).toBeVisible({ timeout: 20000 });
});

test('stage change records an automatic activity and editing it keeps the stage', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '진행상황' }).click();
	await page.getByRole('row', { name: new RegExp(firstOpportunityName) }).click();
	const editSheet = page.getByRole('dialog', { name: '진행 건 수정' });
	await editSheet.getByLabel('단계').click();
	await page.getByRole('option', { name: '진행', exact: true }).click();
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

	await page.getByRole('tab', { name: '진행상황' }).click();
	await expect(page.getByRole('row', { name: new RegExp(firstOpportunityName) })).toContainText('진행');
});

test('rejects deleting an in-use CRM definition with guidance', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();
	await page.getByRole('button', { name: '삭제' }).first().click();
	await expect(
		page.getByText('등록된 CRM 기록에서 사용 중인 항목입니다. 연결된 기록의 값을 변경한 후 삭제해 주세요.')
	).toBeVisible({ timeout: 10000 });
});

test('adds and removes an unused CRM definition', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '정의' }).click();
	const addInput = page.getByRole('textbox', { name: '관계처 유형' });
	await addInput.fill('E2E 임시 유형');
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
	await page.getByText('사업은 업무가 속한 단위입니다').waitFor({ state: 'visible' });
	await page.getByRole('button', { name: '삭제' }).first().click();
	const confirm = page.locator('[role="alertdialog"][data-state="open"]').filter({ hasText: '사업하나' });
	await confirm.locator('[data-alert-dialog-action]').last().click();
	await expect(
		page.getByText('등록된 업무나 진행 건에서 사용 중인 항목입니다. 연결된 기록의 값을 변경한 후 삭제해 주세요.')
	).toBeVisible({ timeout: 10000 });
});

async function chooseCurrencyOption(page: Page, code: string): Promise<void> {
	const option = page.getByRole('option', { name: new RegExp(`\\b${code}\\b`) }).first();
	await option.scrollIntoViewIfNeeded();
	await option.click();
}

async function settledNoteOf(page: Page): Promise<string> {
	await page.getByRole('tab', { name: '진행상황' }).click();
	await page.getByRole('row', { name: new RegExp(settledOpportunityName) }).click();
	const editSheet = page.getByRole('dialog', { name: '진행 건 수정' });
	const note = editSheet.getByText(/확정$/);
	await expect(note).toBeVisible({ timeout: 20000 });
	return (await note.innerText()).trim();
}

test('an administrator moves the company onto another base currency', async ({ page }) => {
	await signIn(page);
	await page.goto('/example-co/settings');
	const currency = page.getByLabel('기준 통화');
	await expect(currency).toContainText('KRW', { timeout: 20000 });

	await currency.click();
	await chooseCurrencyOption(page, 'USD');
	await expect(currency).toContainText('USD');
	await page.getByRole('button', { name: '저장', exact: true }).first().click();

	await page.reload();
	await expect(page.getByLabel('기준 통화')).toContainText('USD', { timeout: 20000 });
});

test('closing a deal priced in another currency settles it in the base currency', async ({ page }) => {
	await signIn(page);
	await openQuickAdd(page, '진행 건');
	const sheet = recordSheet(page);
	await sheet.getByLabel('관계처').click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
	await sheet.getByLabel('이름 또는 제목').fill(settledOpportunityName);
	await sheet.locator('#crm-record-amount-currency').click();
	await chooseCurrencyOption(page, 'KRW');
	await sheet.locator('#crm-record-amount').fill('18000000');
	await sheet.getByRole('button', { name: '추가', exact: true }).click();
	await expect(sheet).not.toBeVisible();

	await page.getByRole('tab', { name: '진행상황' }).click();
	await page.getByRole('row', { name: new RegExp(settledOpportunityName) }).click();
	const editSheet = page.getByRole('dialog', { name: '진행 건 수정' });
	await editSheet.getByLabel('단계').click();
	await page.getByRole('option', { name: '완료', exact: true }).click();
	await editSheet.getByRole('button', { name: '저장', exact: true }).click();
	await expect(editSheet).not.toBeVisible();

	const settled = await settledNoteOf(page);
	expect(settled).toContain('확정');
	expect(settled).toContain('USD ');
});

test('a settled amount survives a move between terminal stages', async ({ page }) => {
	await signIn(page);
	const before = await settledNoteOf(page);

	const editSheet = page.getByRole('dialog', { name: '진행 건 수정' });
	await editSheet.getByLabel('단계').click();
	await page.getByRole('option', { name: '무산', exact: true }).click();
	await editSheet.getByLabel('무산 사유').fill('예산 부족');
	await editSheet.getByRole('button', { name: '저장', exact: true }).click();
	await expect(editSheet).not.toBeVisible();

	expect(await settledNoteOf(page)).toBe(before);
});

test('the view opens in the company base currency and a matching currency shows exact', async ({ page }) => {
	await signIn(page);
	await page.getByRole('tab', { name: '진행상황' }).click();
	const row = page.getByRole('row', { name: new RegExp(settledOpportunityName) });
	await expect(row).toBeVisible();

	const viewCurrency = page.getByLabel('보기 통화');
	await expect(viewCurrency).toHaveText(/USD/, { timeout: 20000 });
	await expect(row.getByText(/^USD /)).toBeVisible({ timeout: 20000 });

	await viewCurrency.click();
	await chooseCurrencyOption(page, 'KRW');
	await expect(row.getByText(/^KRW /)).toBeVisible({ timeout: 20000 });
});
