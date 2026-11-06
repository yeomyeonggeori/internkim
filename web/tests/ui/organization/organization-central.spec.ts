import { expect, test, type Page } from '@playwright/test';
import { exampleCompanySlug, member1ID, member2ID, member3Email, member3ID } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const ownMemberID = member1ID;
const colleagueMemberID = member2ID;
const editedJobTitle = 'E2E 중앙 직책';
const backgroundJobTitle = 'E2E 백그라운드 직책';
const failedJobTitle = 'E2E 저장 실패 직책';
const memberWithoutAdminID = member3ID;
const failedPhoneNumber = '010-0000-0000';
const addedOrganizationName = 'E2E 중앙 조직';

async function signIn(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/organization`);
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
	await expect(panel.getByRole('button', { name: '수정하기' })).toBeEnabled();

	await page.reload();
	await page.getByTestId(`organization-person-node-${ownMemberID}`).waitFor({ state: 'visible', timeout: 20000 });
	await expect(page.getByTestId(`organization-person-node-${ownMemberID}`)).toContainText(editedJobTitle);
});

test('saving closes the edit at once while the save finishes behind it', async ({ page }) => {
	await signIn(page);
	const panel = await openPerson(page, ownMemberID);
	let releaseWrite = () => {};
	const writeHeld = new Promise<void>((resolve) => (releaseWrite = resolve));
	await page.route('**/api/v1/tools/person_update/invoke', async (route) => {
		await writeHeld;
		await route.continue();
	});

	await panel.getByRole('button', { name: '수정하기' }).click();
	await panel.getByLabel('직책', { exact: true }).fill(backgroundJobTitle);
	await panel.getByRole('button', { name: '저장', exact: true }).click();

	await expect(panel.getByLabel('직책', { exact: true })).toHaveCount(0);
	await expect(panel).toContainText(backgroundJobTitle);
	await expect(panel.getByRole('button', { name: '수정하기' })).toBeDisabled();

	const written = page.waitForResponse('**/api/v1/tools/person_update/invoke');
	releaseWrite();
	await written;
	await expect(panel.getByRole('button', { name: '수정하기' })).toBeEnabled();

	await page.reload();
	await page.getByTestId(`organization-person-node-${ownMemberID}`).waitFor({ state: 'visible', timeout: 20000 });
	await expect(page.getByTestId(`organization-person-node-${ownMemberID}`)).toContainText(backgroundJobTitle);
});

test('a save that fails puts the old value back and reopens the edit', async ({ page }) => {
	await signIn(page);
	const panel = await openPerson(page, ownMemberID);
	await page.route('**/api/v1/tools/person_update/invoke', (route) => route.abort());

	await panel.getByRole('button', { name: '수정하기' }).click();
	await panel.getByLabel('직책', { exact: true }).fill(failedJobTitle);
	await panel.getByRole('button', { name: '저장', exact: true }).click();

	await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
	await expect(panel.getByLabel('직책', { exact: true })).toHaveValue(failedJobTitle);
	await expect(page.getByTestId(`organization-person-node-${ownMemberID}`)).not.toContainText(failedJobTitle);
});

test('a member whose own save fails gets their typed phone number back', async ({ page }) => {
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/organization`, member3Email);
	const panel = await openPerson(page, memberWithoutAdminID);
	await page.route('**/api/v1/tools/person_update/invoke', (route) => route.abort());

	await panel.getByRole('button', { name: '수정하기' }).click();
	await panel.getByLabel('전화번호', { exact: true }).fill(failedPhoneNumber);
	await panel.getByRole('button', { name: '저장', exact: true }).click();

	await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
	await expect(panel.getByLabel('전화번호', { exact: true })).toHaveValue(failedPhoneNumber);
});

test('a person can contact a colleague but is offered no way to contact themselves', async ({ page }) => {
	await signIn(page);

	const ownCard = page.getByTestId(`organization-person-card-${ownMemberID}`);
	const colleagueCard = page.getByTestId(`organization-person-card-${colleagueMemberID}`);
	await expect(colleagueCard.getByRole('button', { name: 'DM 보내기' })).toBeVisible();
	await expect(ownCard.getByText('나', { exact: true })).toBeVisible();
	await expect(ownCard.getByRole('button', { name: 'DM 보내기' })).toHaveCount(0);
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

	await expect(panel.getByRole('button', { name: '수정하기', exact: true })).toBeEnabled();
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
