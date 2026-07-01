import { expect, test } from '@playwright/test';
import type { UsersResponse } from './admin-orgchart-fixtures';

const usersResponse: UsersResponse = {
	availableGroups: [
		{ id: 'leadership', name: '경영' },
		{ id: 'product', name: '제품팀' },
		{ id: 'design', name: '디자인팀' },
		{ id: 'field', name: '현장지원팀' }
	],
	records: [
		{
			userID: 'user-ceo',
			handle: 'ceo',
			name: '김도형',
			email: 'ceo@example.com',
			hireDate: '2026-01-03',
			role: 'member',
			jobTitle: '대표이사',
			primaryGroupID: 'leadership',
			groupIDs: ['leadership']
		},
		{
			userID: 'user-junho',
			handle: 'junho',
			name: '이정훈',
			email: 'junho@example.com',
			hireDate: '2026-02-10',
			role: 'member',
			jobTitle: '제품팀 리드',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-ceo'
		},
		{
			userID: 'user-dabin',
			handle: 'dabin',
			name: '김다빈',
			email: 'dabin@example.com',
			hireDate: '2026-03-11',
			role: 'member',
			jobTitle: '프론트엔드 개발자',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-junho'
		},
		{
			userID: 'user-minjae',
			handle: 'minjae',
			name: '강민재',
			email: 'minjae@example.com',
			hireDate: '2026-03-13',
			role: 'member',
			jobTitle: '백엔드 개발자',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-junho'
		},
		{
			userID: 'user-taehyun',
			handle: 'taehyun',
			name: '신태현',
			email: 'taehyun@example.com',
			hireDate: '2026-03-20',
			role: 'member',
			jobTitle: 'QA 엔지니어',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-dabin'
		},
		{
			userID: 'user-jieun',
			handle: 'jieun',
			name: '박지은',
			email: 'jieun@example.com',
			hireDate: '2026-02-12',
			role: 'member',
			jobTitle: '디자인 리드',
			primaryGroupID: 'design',
			groupIDs: ['design'],
			supervisorID: 'user-ceo'
		},
		{
			userID: 'user-nam',
			handle: 'nam',
			name: '남지훈',
			email: 'nam@example.com',
			hireDate: '2026-04-01',
			role: 'member',
			jobTitle: '사업 개발',
			groupIDs: [],
			supervisorID: 'user-ceo'
		}
	]
};

test.describe('employee orgchart directory', () => {
	test('opens from the app shell and filters read-only org chart profiles', async ({ page }) => {
		await mockOrgchartDirectory(page);

		await page.goto('/orgchart/');

		await expect(page.getByRole('link', { name: '조직도' }).first()).toBeVisible();
		await expect(page.getByRole('button', { name: '조직도' })).toBeVisible();
		await expect(page.getByTestId('orgchart-canvas')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-ceo')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-product')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-__unassigned__')).toBeVisible();
		await expect(page.getByTestId('orgchart-tree-node-user-taehyun')).toBeVisible();
		await expect(page.getByText('100%')).toBeVisible();
		await expect(page.getByTestId('orgchart-connector-layer')).toBeVisible();
		await expect(page.getByRole('link', { name: '인사 정보 수정' })).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-detail-panel')).toHaveCount(0);
		await expectOrgchartTeamGridToFillCanvas(page);
		await expect(page.getByTestId('orgchart-canvas').getByText(/^팀 리드:/)).toHaveCount(0);
		await expect(page.getByTestId('orgchart-canvas').getByText(/^구성원 1명$/)).toHaveCount(0);

		await page.getByTestId('orgchart-person-node-user-dabin').click();
		await expect(page.getByTestId('orgchart-person-detail-panel')).toContainText('dabin@example.com');
		await expect(page.getByTestId('orgchart-person-detail-panel')).toContainText('프론트엔드 개발자');
		await expect(page.getByTestId('orgchart-person-detail-panel').getByText('계정 권한 관리는 관리자 탭에서 관리해 주세요.')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-detail-panel').getByRole('link', { name: '인사 정보 수정' })).toHaveCount(0);
		await page.getByRole('button', { name: '상세 닫기' }).click();
		await expect(page.getByTestId('orgchart-person-detail-panel')).toHaveCount(0);

		await page.getByLabel('검색').fill('없는직원');
		await expect(page.getByText('표시할 조직도 구성원이 없습니다.')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toHaveCount(0);

		await page.getByLabel('검색').fill('프론트');
		await expect(page.getByTestId('orgchart-person-node-user-ceo')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toBeVisible();

		await page.getByRole('button', { name: '필터' }).click();
		await page.getByRole('button', { name: '조직', exact: true }).click();
		await page.getByRole('option', { name: '제품팀' }).click();
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toBeVisible();

		await page.getByRole('button', { name: '직원 목록' }).click();
		await expect(page.getByTestId('orgchart-people-list')).toBeVisible();
		await expect(page.getByTestId('orgchart-list-row-user-dabin')).toBeVisible();
	});
});

async function expectOrgchartTeamGridToFillCanvas(page: import('@playwright/test').Page): Promise<void> {
	const [teamGridBox, connectorBox, canvasBox] = await Promise.all([
		page.getByTestId('orgchart-team-grid').boundingBox(),
		page.getByTestId('orgchart-connector-layer').boundingBox(),
		page.getByTestId('orgchart-canvas').boundingBox()
	]);
	const teamGridWidth = teamGridBox?.width ?? 0;
	const connectorWidth = connectorBox?.width ?? 0;
	const canvasWidth = canvasBox?.width ?? 0;
	expect(teamGridWidth).toBeGreaterThanOrEqual(canvasWidth * 0.92);
	expect(Math.abs(teamGridWidth - connectorWidth)).toBeLessThanOrEqual(1);
	expect(Math.abs((teamGridBox?.x ?? 0) - (connectorBox?.x ?? 0))).toBeLessThanOrEqual(1);
}

async function mockOrgchartDirectory(page: import('@playwright/test').Page): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'dabin@example.com' } });
	});
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ status: 403, body: 'admin access required' });
	});
	await page.route('**/orgchart/api/people', async (route) => {
		await route.fulfill({ json: usersResponse });
	});
}
