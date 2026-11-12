import { expect, test, type Page } from '@playwright/test';
import { exampleCompanySlug, member3Email } from '../support/central-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { subscribeTheCompany, unsubscribeTheCompany } from './company-subscription';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const settingsPath = `/${exampleCompanySlug}/settings`;
const memberMenu = ['로그인·보안', '알림', '내 에이전트', '개발자', '호스트'];
const subscribedAdministratorMenu = ['로그인·보안', '알림', '내 에이전트', '개발자', '사용량', '기본 정보', '멤버', '근태', '에이전트', '결제', '호스트'];

function settingsMenu(page: Page) {
	return page.getByRole('navigation', { name: '설정', exact: true });
}

test.afterAll(unsubscribeTheCompany);

test('a member of a company that does not subscribe sees the general menu with the host and without usage', async ({ page }) => {
	await signInToTheCentralPlane(page, settingsPath, member3Email);
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/security$`), { timeout: 20000 });
	await expect(settingsMenu(page).getByRole('link')).toHaveText(memberMenu, { timeout: 20000 });
	await expect(settingsMenu(page).getByText('관리자', { exact: true })).toHaveCount(0);
	await settingsMenu(page).getByRole('link', { name: '호스트', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/host$`));
	await expect(page.getByRole('heading', { level: 1, name: '호스트' })).toBeVisible();
});

test('a member who opens an administrator page or a hidden usage page is sent back to settings', async ({ page }) => {
	await signInToTheCentralPlane(page, `${settingsPath}/members`, member3Email);
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/security$`), { timeout: 20000 });
	await expect(page.getByRole('button', { name: '구성원 초대', exact: true })).toHaveCount(0);
	await page.goto(`${settingsPath}/usage`);
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/security$`), { timeout: 20000 });
});

test('a section read ahead on entering settings opens without waiting for its request', async ({ page }) => {
	const tokensReadAhead = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/v1/tokens', { timeout: 20000 });
	await signInToTheCentralPlane(page, settingsPath, member3Email);
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/security$`), { timeout: 20000 });
	await tokensReadAhead;

	await page.route('**/api/v1/tokens', () => {});
	await settingsMenu(page).getByRole('link', { name: '개발자', exact: true }).click();
	await expect(page.getByRole('heading', { level: 1, name: '개발자' })).toBeVisible();
	await expect(page.getByText('아직 만든 토큰이 없습니다.', { exact: true })).toBeVisible();
	await expect(page.getByRole('status', { name: '개인 액세스 토큰', exact: true })).toHaveCount(0);
});

test('on a phone settings is a list, an entry opens its page, and back returns to the list', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheCentralPlane(page, settingsPath, member3Email);
	const list = page.getByRole('main').getByRole('navigation', { name: '설정', exact: true });
	await expect(list.getByRole('link')).toHaveText(memberMenu, { timeout: 20000 });
	await expect(page).toHaveURL(new RegExp(`${settingsPath}$`));

	await list.getByRole('link', { name: '알림', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/notifications$`));
	await expect(page.getByRole('heading', { level: 1, name: '알림' })).toBeVisible();
	await page.goBack();
	await expect(page).toHaveURL(new RegExp(`${settingsPath}$`));
	await expect(list.getByRole('link', { name: '알림', exact: true })).toBeVisible();

	await list.getByRole('link', { name: '개발자', exact: true }).click();
	await expect(page.getByRole('heading', { level: 1, name: '개발자' })).toBeVisible();
	await page.getByRole('main').getByRole('link', { name: '설정', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`${settingsPath}$`));
	await expect(list.getByRole('link', { name: '개발자', exact: true })).toBeVisible();
});

test('an administrator of a subscribed company sees five general and six administrator entries', async ({ page }) => {
	await subscribeTheCompany();
	await signInToTheCentralPlane(page, settingsPath);
	await expect(page).toHaveURL(new RegExp(`${settingsPath}/security$`), { timeout: 20000 });
	await expect(settingsMenu(page).getByRole('link')).toHaveText(subscribedAdministratorMenu, { timeout: 20000 });
	await expect(settingsMenu(page).getByText('일반', { exact: true })).toBeVisible();
	await expect(settingsMenu(page).getByText('관리자', { exact: true })).toBeVisible();
	await settingsMenu(page).getByRole('link', { name: '호스트', exact: true }).click();
	await expect(page.getByText('회사 컴퓨터 연결', { exact: true })).toBeVisible({ timeout: 20000 });
	await expect(page.getByRole('heading', { level: 2, name: '김인턴에게 말 걸기', exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: '메신저 열기', exact: true })).toBeVisible();
	await expect(page.getByRole('link', { name: '김인턴 시작하기' })).toHaveCount(0);
});
