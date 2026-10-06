import { expect, test, type Locator, type Page, type Route } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { AttendanceLoadingFixture, LoadingResponseGate, loadingFixtureTime } from './attendance-loading-fixture';

const phase = process.env.EMPTY_CAPTURE_PHASE ?? 'after';
const output = process.env.EMPTY_CAPTURE_DIR;
const fixtureFailure = '샘플 응답을 불러오지 못했습니다';

class AttendanceEmptyFixture extends AttendanceLoadingFixture {
	emptyTeams = false;
	emptyLeave = true;
	emptyChanges = true;
	readonly holidays = new LoadingResponseGate();

	async answer(route: Route): Promise<void> {
		const tool = new URL(route.request().url()).pathname.split('/').at(-2);
		const posted: { input: Record<string, unknown> } = route.request().postDataJSON();
		const input = posted.input;
		if (tool === 'leave_list' && this.emptyLeave) {
			return this.reply(route, this.leave, { count: 0, leave: [], registeredKinds: ['annual'] });
		}
		if (tool === 'attendance_changes_page_get' && this.emptyChanges) {
			return this.reply(route, this.changes, { attendance: [], totalCount: 0 });
		}
		if (tool === 'company_holiday_list') {
			return this.reply(route, this.holidays, { count: 0, year: null, holidays: [] });
		}
		if ((tool === 'attendance_team_dashboard_get' || tool === 'attendance_team_page_get') &&
			((input.pageKind === 'teams' && this.emptyTeams) ||
				(input.pageKind === 'members' && Boolean(input.searchText || input.locationFilter)))) {
			return this.reply(route, input.pageKind === 'members' ? this.members : this.teams, {
				companyID: this.identity.companyID, companyName: '샘플컴퍼니',
				timeZone: 'Asia/Seoul', serverTime: loadingFixtureTime,
				authorization: { isAdmin: true, teamViewVisibleToAll: true },
				teamOffset: 0, teamLimit: 6, teamTotal: 0, teams: [],
				selectedTeamKey: input.selectedTeamKey || null,
				memberOffset: 0, memberLimit: 24, memberTotal: 0, members: []
			});
		}
		return super.answer(route);
	}

	private async reply(route: Route, gate: LoadingResponseGate, result: unknown): Promise<void> {
		await gate.wait();
		if (gate.fail) {
			await route.fulfill({ status: 503, json: { error: fixtureFailure } });
			return;
		}
		await route.fulfill({ json: { result } });
	}
}

type Scene = {
	name: string;
	label: string;
	testID: string;
	admin: boolean;
	gate: 'leave' | 'changes';
	empty: string;
	error: string;
};

const scenes: Scene[] = [
	{ name: 'leave-history', label: '내 휴가', testID: 'leave-history-view', admin: false, gate: 'leave', empty: '표시할 휴가 내역이 없습니다.', error: '휴가 정보를 불러오지 못했습니다.' },
	{ name: 'approvals', label: '휴가 승인', testID: 'leave-approval-view', admin: true, gate: 'leave', empty: '승인 대기 중인 휴가가 없습니다.', error: '휴가 승인 목록을 불러오지 못했습니다.' },
	{ name: 'manual-history', label: '수정 내역', testID: 'hand-written-view', admin: true, gate: 'changes', empty: '수정 내역이 없습니다.', error: '수정 내역을 불러오지 못했습니다.' }
];

async function openScene(page: Page, scene: Scene, width: number): Promise<Locator> {
	await page.goto('/example-co/attendance');
	await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
	if (width < 768 && scene.admin) {
		await page.getByRole('button', { name: '관리', exact: true }).click();
		await page.getByRole('menuitem', { name: scene.label, exact: true }).click();
	} else {
		await page.getByRole('button', { name: scene.label, exact: true }).filter({ visible: true }).click();
	}
	const scope = page.getByTestId(scene.testID);
	await expect(scope).toBeVisible();
	return scope;
}

async function capture(page: Page, scene: string, width: number, target?: Locator): Promise<void> {
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
	if (!output) return;
	await mkdir(`${output}/${phase}`, { recursive: true });
	await page.evaluate(async () => { await document.fonts.ready; });
	if (target) await target.scrollIntoViewIfNeeded();
	await page.screenshot({ path: `${output}/${phase}/attendance-${scene}-${width}.png`, animations: 'disabled' });
}

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

for (const width of [1280, 390]) {
	for (const scene of scenes) {
		test(`attendance ${scene.name} distinguishes pending and loaded empty at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new AttendanceEmptyFixture();
			const gate = fixture[scene.gate];
			gate.block();
			await fixture.install(page);
			const scope = await openScene(page, scene, width);
			await expect.poll(() => gate.reads).toBeGreaterThan(0);
			await expect(scope.locator('[data-slot="skeleton"]').first()).toBeVisible();
			await expect(scope.getByText(scene.empty, { exact: true })).toHaveCount(0);
			await expect(scope.locator('[data-slot="empty"]')).toHaveCount(0);
			await capture(page, `${scene.name}-pending`, width);
			gate.release();
			await expect(scope.getByText(scene.empty, { exact: true })).toBeVisible();
			await expect(scope.locator('[data-slot="skeleton"]')).toHaveCount(0);
			if (phase !== 'before') await expect(scope.locator('[data-slot="empty-title"]').filter({ hasText: scene.empty })).toBeVisible();
			await capture(page, `${scene.name}-empty`, width);

			if (scene.name === 'manual-history') {
				await scope.getByRole('button', { name: '팀', exact: true }).click();
				await page.getByRole('option', { name: '제품개발팀', exact: true }).click();
				await scope.getByRole('button', { name: '조회', exact: true }).click();
				await expect(scope.getByTestId('hand-written-empty')).toBeVisible();
				if (phase !== 'before') await expect(scope.getByTestId('hand-written-empty')).toHaveText('선택한 기간과 조건에 맞는 수정 내역이 없습니다.');
				await capture(page, 'manual-history-filtered-empty', width);
			}
		});

		test(`attendance ${scene.name} initial failure is never empty at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new AttendanceEmptyFixture();
			fixture[scene.gate].fail = true;
			await fixture.install(page);
			const scope = await openScene(page, scene, width);
			await expect(scope.getByText(scene.error, { exact: true }).first()).toBeVisible();
			await expect(scope.getByText(scene.empty, { exact: true })).toHaveCount(0);
			await expect(scope.locator('[data-slot="empty"]')).toHaveCount(0);
			await expect(scope.locator('[data-slot="skeleton"]')).toHaveCount(0);
			await capture(page, `${scene.name}-error`, width);
		});
	}

	test(`attendance zero teams is a settled empty state at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new AttendanceEmptyFixture();
		fixture.emptyTeams = true;
		fixture.teams.block();
		await fixture.install(page);
		await page.goto('/example-co/attendance');
		const scope = page.getByTestId('attendance-team-dashboard');
		await expect(scope.locator('[data-attendance-skeleton="teams"]')).toBeVisible();
		await expect(scope.locator('[data-slot="empty"]')).toHaveCount(0);
		fixture.teams.release();
		await expect(scope.locator('[data-attendance-skeleton="teams"]')).toHaveCount(0);
		await expect(scope.getByTestId('attendance-team-card')).toHaveCount(0);
		if (phase !== 'before') await expect(scope.locator('[data-slot="empty-title"]')).toHaveText('표시할 팀이 없습니다.');
		await capture(page, 'teams-empty', width);
	});

	test(`attendance filtered members can clear filters at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new AttendanceEmptyFixture();
		await fixture.install(page);
		await page.goto('/example-co/attendance');
		await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
		await page.getByTestId('attendance-team-card').first().getByRole('button', { name: /구성원|직원/ }).click();
		const members = page.getByTestId('team-employee-page');
		await expect(members.getByRole('button')).toHaveCount(8);
		const search = page.getByRole('textbox', { name: '이름 또는 이메일 검색', exact: true });
		fixture.members.block();
		await search.fill('matching-nobody');
		await expect(page.locator('[data-attendance-skeleton="members"]')).toBeVisible();
		await expect(members.locator('[data-slot="empty"]')).toHaveCount(0);
		fixture.members.release();
		await expect(members.getByText(phase === 'before' ? '표시할 구성원이 없습니다.' : '검색 조건에 맞는 구성원이 없습니다.', { exact: true })).toBeVisible();
		if (phase !== 'before') await expect(members.locator('[data-slot="empty"]')).toBeVisible();
		await capture(page, 'members-filtered-empty', width);
		if (phase === 'before') await search.fill('');
		else await members.getByRole('button', { name: '필터 초기화', exact: true }).click();
		await expect(search).toHaveValue('');
		await expect(members.getByRole('button')).toHaveCount(8);
		await expect(members.locator('[data-slot="empty"]')).toHaveCount(0);
	});

	test(`company holiday read failure survives Add and Cancel at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new AttendanceEmptyFixture();
		fixture.holidays.fail = true;
		await fixture.install(page);
		await page.route('**/api/v1/tokens', route => route.fulfill({ json: { tokens: [] } }));
		await page.goto('/example-co/settings');
		await page.getByRole('tab', { name: '관리자', exact: true }).click();
		const holidays = page.getByTestId('company-holiday-settings');
		await expect(holidays.getByText(fixtureFailure, { exact: true })).toBeVisible();
		if (phase !== 'before') await expect(holidays.getByText('등록된 회사 휴일이 없습니다.', { exact: true })).toHaveCount(0);
		await capture(page, 'holidays-error', width, holidays);
		await holidays.getByRole('button', { name: '휴일 추가', exact: true }).click();
		await expect(holidays.getByTestId('company-holiday-editor')).toBeVisible();
		await holidays.getByRole('button', { name: '취소', exact: true }).click();
		await expect(holidays.getByTestId('company-holiday-editor')).toHaveCount(0);
		if (phase !== 'before') {
			await expect(holidays.getByRole('alert')).toHaveText(fixtureFailure);
			await expect(holidays.locator('[data-slot="empty"]')).toHaveCount(0);
			await expect(holidays.getByText('등록된 회사 휴일이 없습니다.', { exact: true })).toHaveCount(0);
		}
		await capture(page, 'holidays-error-after-cancel', width, holidays);
	});
}
