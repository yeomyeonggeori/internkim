import { expect, test, type Page } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type { UpdateAttendanceEventRequest } from '../../src/routes/attendance/attendance-api';
import type {
	AttendanceEvent,
	AttendanceSummary
} from '../../src/routes/attendance/attendance-context.svelte';

test.describe('attendance', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com', isAdmin: true } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
		});
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? '2026-05';
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});
	});

	test('renders the monthly team status table with personal tools in the fixed sidebar', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);

		await expect(page.getByRole('tab')).toHaveCount(0);
		await expect(page.getByTestId('team-month-calendar')).toHaveCount(0);
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('team-status-table')).toBeVisible();
		await expect(page.getByText('월간 근무 현황표')).toBeVisible();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByTestId('personal-month-calendar-grid')).toBeVisible();
		await expect(page.getByRole('button', { name: '부재 등록' })).toBeVisible();

		const statusTable = page.getByTestId('team-status-table');
		await page.getByPlaceholder('직원 검색').fill('김철수');
		await expect(statusTable.getByText('김철수')).toBeVisible();
		await expect(statusTable.getByText('강민호')).toHaveCount(0);
	});

	test('shows location bars and segment tooltip in a team status cell', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);

		const todayCell = page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`);
		await expect(todayCell.getByText('외부')).toBeVisible();
		await expect(todayCell.getByText('근무 중')).toHaveCount(0);
		await todayCell.hover();
		await expect(page.getByText('08:30-10:20')).toBeVisible();
		await expect(page.getByText('1시간 50분')).toBeVisible();
		await expect(page.getByText('10:45-12:20')).toBeVisible();
		await expect(page.getByText('1시간 35분')).toBeVisible();
		await expect(page.getByText('12:45~')).toBeVisible();
		await expect(page.getByText('진행 중')).toBeVisible();
	});

	test('opens status day details from a team status cell', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(dialog.getByText('김철수')).toBeVisible();
		await expect(dialog.getByText('상태')).toHaveCount(0);
		await expect(dialog.getByText('3시간 25분')).toBeVisible();
		await expect(dialog.getByText('재택')).toBeVisible();
		await expect(dialog.getByText('사무실')).toBeVisible();
		await expect(dialog.getByText('외부')).toBeVisible();
		await expect(dialog.getByText('08:30-10:20')).toBeVisible();
		await expect(dialog.getByText('10:45-12:20')).toBeVisible();
		await expect(dialog.getByText('12:45~')).toBeVisible();
	});

	test('shows selected personal day details from the sidebar calendar', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`personal-calendar-day-${todayDate}`).click();

		const detailPanel = page.getByTestId('personal-day-detail-panel');
		await expect(detailPanel.getByText('3구간')).toBeVisible();
		await expect(detailPanel.getByText('08:30~10:20')).toBeVisible();
		await expect(detailPanel.getByText('10:45~12:20')).toBeVisible();
		await expect(detailPanel.getByText('12:45~')).toBeVisible();

		const eventLabels = detailPanel.getByTestId('personal-day-event-label');
		await expect(eventLabels.nth(0)).toContainText('출근 08:30');
		await expect(eventLabels.nth(1)).toContainText('퇴근 10:20');
		await expect(eventLabels.nth(2)).toContainText('출근 10:45');
		await expect(eventLabels.nth(3)).toContainText('퇴근 12:20');
		await expect(eventLabels.nth(4)).toContainText('출근 12:45');
	});

	test('registers own absence without sending an email override', async ({ page }) => {
		let requestBody: Record<string, unknown> = {};
		await page.route('**/attendance/api/absences', async (route) => {
			requestBody = JSON.parse(route.request().postData() ?? '{}') as Record<string, unknown>;
			await route.fulfill({
				json: {
					absences: [
						{
							id: 'absence-created',
							email: 'kim@example.com',
							kind: 'leave',
							labelKey: 'leave',
							date: '2026-06-10',
							createdAt: '2026-06-10T09:00:00+09:00'
						}
					]
				}
			});
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByLabel('시작일').fill('2026-06-10');
		await page.getByLabel('종료일').fill('2026-06-10');
		await page.getByLabel('사유').fill('family');
		await page.getByRole('button', { name: '부재 등록' }).click();

		await expect(page.getByText('부재를 등록했습니다.')).toBeVisible();
		expect(requestBody).toEqual({
			kind: 'leave',
			startDate: '2026-06-10',
			endDate: '2026-06-10',
			reason: 'family'
		});
	});

	test('shows a weekday-only notice when a weekend absence creates no records', async ({ page }) => {
		await page.route('**/attendance/api/absences', async (route) => {
			await route.fulfill({ json: { absences: [] } });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByLabel('시작일').fill('2026-06-13');
		await page.getByLabel('종료일').fill('2026-06-14');
		await page.getByRole('button', { name: '부재 등록' }).click();

		await expect(page.getByText('등록할 평일이 없습니다.')).toBeVisible();
		await expect(page.getByText('부재를 등록했습니다.')).toHaveCount(0);
	});

	test('updates an attendance event only after saving an override reason', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		let summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const targetEvent = currentUserEvent(summary, todayDate, 'clock_in');
		if (!targetEvent) throw new Error('Expected a current user clock-in event fixture');

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: summary });
		});
		await page.route('**/attendance/api/events/*', async (route) => {
			const request = parseUpdateAttendanceEventRequest(route.request().postData());
			const eventID = decodeURIComponent(route.request().url().split('/').at(-1) ?? '');
			summary = replaceSummaryEvent(summary, eventID, request);
			await route.fulfill({ json: { ok: true } });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`personal-calendar-day-${todayDate}`).click();

		const detailPanel = page.getByTestId('personal-day-detail-panel');
		const eventRow = detailPanel
			.getByTestId('personal-day-event-row')
			.filter({ has: page.getByTestId('personal-day-event-label').filter({ hasText: '출근 08:30' }) });
		await eventRow.getByTestId('personal-day-event-label').click();
		await eventRow.getByRole('button', { name: '수정' }).click();
		await eventRow.getByLabel('시간').fill('08:40');
		await eventRow.getByLabel('장소').selectOption('office');
		await eventRow.getByLabel('수정 사유').fill('시간 보정');
		await eventRow.getByRole('button', { name: '저장' }).click();

		const updatedEventRow = detailPanel
			.getByTestId('personal-day-event-row')
			.filter({ has: page.getByTestId('personal-day-event-label').filter({ hasText: '출근 08:40' }) });
		await expect(updatedEventRow.getByTestId('personal-day-event-label')).toContainText('사무실');
		await expect(updatedEventRow.getByText('수정 전:')).toBeVisible();
		await expect(updatedEventRow.getByText('08:30 · 재택')).toBeVisible();
		await expect(updatedEventRow.getByText('수정 후:')).toBeVisible();
		await expect(updatedEventRow.getByText('08:40 · 사무실', { exact: true })).toBeVisible();
		await expect(updatedEventRow.getByText('수정 사유:')).toBeVisible();
		await expect(updatedEventRow.getByText('시간 보정')).toBeVisible();
	});

	test('cancels an own absence from the selected day panel', async ({ page }) => {
		let summary = buildAttendanceSummaryFixture('2026-06');
		const targetAbsence = summary.absences.find(
			(absence) => absence.email === summary.currentUserEmail && absence.kind === 'leave'
		);
		if (!targetAbsence) throw new Error('Expected a current user leave absence fixture');
		let deletedAbsenceID = '';

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? summary.month;
			await route.fulfill({ json: month === summary.month ? summary : buildAttendanceSummaryFixture(month) });
		});
		await page.route('**/attendance/api/absences/*', async (route) => {
			deletedAbsenceID = decodeURIComponent(route.request().url().split('/').at(-1) ?? '');
			summary = {
				...summary,
				absences: summary.absences.filter((absence) => absence.id !== deletedAbsenceID)
			};
			await route.fulfill({ json: { ok: true } });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`personal-calendar-day-${targetAbsence.date}`).click();

		const detailPanel = page.getByTestId('personal-day-detail-panel');
		await expect(detailPanel.getByText('휴가')).toBeVisible();
		await detailPanel.getByRole('button', { name: '취소' }).click();

		expect(deletedAbsenceID).toBe(targetAbsence.id);
		await expect(detailPanel.getByText('휴가')).toHaveCount(0);
	});
});

async function selectKorean(page: Page): Promise<void> {
	await page.getByRole('button', { name: /Change language|언어 변경/ }).click();
	await page.getByRole('menuitemradio', { name: '한국어' }).click();
}

function todayDateInSeoul(): string {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).formatToParts(new Date());
	const dateParts = Object.fromEntries(parts.map((part) => [part.type, part.value]));
	return `${dateParts.year}-${dateParts.month}-${dateParts.day}`;
}

function currentUserEvent(
	summary: AttendanceSummary,
	localDate: string,
	kind: AttendanceEvent['kind']
): AttendanceEvent | undefined {
	return summary.events.find(
		(event) => event.email === summary.currentUserEmail && event.localDate === localDate && event.kind === kind
	);
}

function parseUpdateAttendanceEventRequest(payload: string | null): UpdateAttendanceEventRequest {
	const parsed = JSON.parse(payload ?? '{}') as Partial<UpdateAttendanceEventRequest>;
	if (
		typeof parsed.localDate !== 'string' ||
		typeof parsed.localTime !== 'string' ||
		typeof parsed.locationID !== 'string' ||
		typeof parsed.reason !== 'string'
	) {
		throw new Error('Invalid update attendance event request');
	}
	return {
		localDate: parsed.localDate,
		localTime: parsed.localTime,
		locationID: parsed.locationID,
		reason: parsed.reason
	};
}

function replaceSummaryEvent(
	summary: AttendanceSummary,
	eventID: string,
	request: UpdateAttendanceEventRequest
): AttendanceSummary {
	return {
		...summary,
		events: summary.events.map((event) => (event.id === eventID ? overrideEvent(summary, event, request) : event))
	};
}

function overrideEvent(
	summary: AttendanceSummary,
	event: AttendanceEvent,
	request: UpdateAttendanceEventRequest
): AttendanceEvent {
	const location = summary.locations.find((candidate) => candidate.id === request.locationID);
	const locationName = location?.name ?? request.locationID;
	const editedAt = `${request.localDate}T17:20:00+09:00`;
	return {
		...event,
		occurredAt: `${request.localDate}T${request.localTime}:00+09:00`,
		localDate: request.localDate,
		localTime: request.localTime,
		locationID: request.locationID,
		locationName,
		overriddenBy: summary.currentUserEmail,
		overriddenAt: editedAt,
		overrideReason: request.reason,
		originalOccurredAt: event.occurredAt,
		originalLocalDate: event.localDate,
		originalLocalTime: event.localTime,
		originalLocationID: event.locationID,
		originalLocationName: event.locationName,
		overrideHistory: [
			{
				id: `override-${event.id}`,
				eventID: event.id,
				editedBy: summary.currentUserEmail,
				editedAt,
				reason: request.reason,
				originalOccurredAt: event.occurredAt,
				originalLocalDate: event.localDate,
				originalLocalTime: event.localTime,
				originalLocationID: event.locationID ?? '',
				originalLocationName: event.locationName ?? '',
				overrideOccurredAt: `${request.localDate}T${request.localTime}:00+09:00`,
				overrideLocalDate: request.localDate,
				overrideLocalTime: request.localTime,
				overrideLocationID: request.locationID,
				overrideLocationName: locationName
			},
			...(event.overrideHistory ?? [])
		]
	};
}
