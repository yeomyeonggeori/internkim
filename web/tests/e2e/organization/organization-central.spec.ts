import { expect, test, type Page } from '@playwright/test';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const ownMemberID = '000000ee-0000-0000-0000-000000000001';
const colleagueMemberID = '000000ee-0000-0000-0000-000000000002';
const editedJobTitle = 'E2E 중앙 직책';
const addedOrganizationName = 'E2E 중앙 조직';

async function signIn(page: Page): Promise<void> {
	await page.goto('/example-co/organization');
	const email = page.getByRole('textbox', { name: '이메일' });
	const needsSignIn = await email
		.waitFor({ state: 'visible', timeout: 8000 })
		.then(() => true)
		.catch(() => false);
	if (needsSignIn) {
		await email.fill('member1@example.com');
		await page.getByRole('textbox', { name: '비밀번호' }).fill('seed-password');
		await page.getByRole('button', { name: '로그인', exact: true }).click();
	}
	await page.getByTestId(`organization-person-node-${ownMemberID}`).waitFor({ state: 'visible', timeout: 20000 });
}

function detailPanel(page: Page) {
	return page.getByTestId('organization-person-detail-panel');
}

async function organizationIDInTree(page: Page, organizationName: string): Promise<string> {
	const row = page.locator('[data-testid^="organization-row-"]', {
		has: page.getByRole('button', { name: organizationName, exact: true })
	});
	await expect(row).toBeVisible();
	const organizationID = (await row.getAttribute('data-testid'))?.replace('organization-row-', '') ?? '';
	expect(organizationID).not.toBe('');
	return organizationID;
}

async function openPerson(page: Page, memberID: string) {
	await page.getByTestId(`organization-person-node-${memberID}`).click();
	const panel = detailPanel(page);
	await expect(panel).toBeVisible();
	return panel;
}

test('an administrator saves their own job title', async ({ page }) => {
	await signIn(page);
	const panel = await openPerson(page, ownMemberID);

	await panel.getByRole('button', { name: '수정하기' }).click();
	await panel.getByLabel('직책', { exact: true }).fill(editedJobTitle);
	await panel.getByRole('button', { name: '저장', exact: true }).click();

	await expect(panel).toContainText(editedJobTitle);
	await expect(page.getByText('저장하지 않은 조직도 변경사항이 있습니다.')).toHaveCount(0);

	await page.reload();
	await page.getByTestId(`organization-person-node-${ownMemberID}`).waitFor({ state: 'visible', timeout: 20000 });
	await expect(page.getByTestId(`organization-person-node-${ownMemberID}`)).toContainText(editedJobTitle);
});

test('an administrator moves a colleague into a newly added organization', async ({ page }) => {
	await signIn(page);

	await page.getByRole('button', { name: '조직 작업' }).first().click();
	await page.getByRole('menuitem', { name: '조직 추가' }).click();
	await page.getByLabel('새 조직').fill(addedOrganizationName);
	await page.getByRole('button', { name: '추가', exact: true }).click();
	await expect(page.getByRole('dialog', { name: '조직 추가', exact: true })).toBeHidden();

	await page.reload();
	const addedOrganizationID = await organizationIDInTree(page, addedOrganizationName);
	const panel = await openPerson(page, colleagueMemberID);
	await panel.getByRole('button', { name: '수정하기' }).click();
	await panel.getByLabel('소속 조직').click();
	await page.getByRole('option', { name: addedOrganizationName, exact: true }).click();
	await panel.getByRole('button', { name: '저장', exact: true }).click();

	await expect(panel.getByRole('button', { name: '수정하기', exact: true })).toBeVisible();
	await expect(panel).toContainText(addedOrganizationName);

	await page.reload();
	await expect(
		page
			.getByTestId(`organization-members-${addedOrganizationID}`)
			.getByTestId(`organization-person-node-${colleagueMemberID}`)
	).toBeVisible();
});

test('closing an unsaved edit asks before it throws the edit away', async ({ page }) => {
	await signIn(page);
	const panel = await openPerson(page, ownMemberID);

	await panel.getByRole('button', { name: '수정하기' }).click();
	await panel.getByLabel('직책', { exact: true }).fill('저장하지 않은 직책');
	await page.getByRole('dialog', { name: '직원 상세' }).getByRole('button', { name: '상세 닫기' }).click();

	const dialog = page.getByTestId('organization-discard-edits-dialog');
	await expect(dialog).toBeVisible();
	await dialog.getByRole('button', { name: '계속 수정', exact: true }).click();
	await expect(panel).toBeVisible();
	await expect(panel.getByLabel('직책', { exact: true })).toHaveValue('저장하지 않은 직책');

	await page.getByRole('dialog', { name: '직원 상세' }).getByRole('button', { name: '상세 닫기' }).click();
	await page.getByTestId('organization-discard-edits-dialog').getByRole('button', { name: '닫기', exact: true }).click();
	await expect(detailPanel(page)).toHaveCount(0);
});
