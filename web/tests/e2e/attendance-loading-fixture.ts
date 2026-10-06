import type { Page, Route } from '@playwright/test';
import { defaultLeavePolicy } from '../../src/lib/attendance/leave-policy-defaults';

export const loadingFixtureTime = '2026-10-06T03:00:00.000Z';
const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const people = Array.from({ length: 8 }, (_, index) => ({
	personID: index === 0 ? memberID : `sample-${index}`, name: ['이샘플', '박예시', '최견본'][index % 3] + (index + 1),
	email: index === 0 ? 'member@example.com' : `sample${index}@example.com`, isAdmin: index === 0
}));
const workLocations = [{ name: '사무실', color: null }, { name: '재택', color: null }];
const ownEvent = { id: 'sample-clock-in', personID: memberID, kind: 'clock_in', occurredAt: '2026-10-05T23:00:00.000Z', location: '사무실' };
const teams = ['제품개발팀', '디자인팀', '사업운영팀', '고객경험팀', '마케팅팀', '경영지원팀'].map((name, index) => ({
	teamKey: `team-${index}`, name, memberCount: 8, working: 5, done: 1, away: 1, notStarted: 1, needsCheckout: 0,
	recentClockIns: people.slice(0, 4).map(person => ({ memberID: person.personID, name: person.name, email: person.email, occurredAt: ownEvent.occurredAt, location: '사무실' })),
	recentClockOuts: [{ memberID: 'sample-1', name: '박예시2', email: 'sample1@example.com', occurredAt: '2026-10-06T01:00:00.000Z', location: '사무실' }],
	recordedLocations: [{ name: '사무실', count: 3 }, { name: '재택', count: 2 }], unknownLocationCount: 0
}));

export class LoadingResponseGate {
	private resolve: () => void = () => {};
	private waiting = Promise.resolve();
	blocked = false;
	reads = 0;
	fail = false;
	block(): void {
		this.blocked = true;
		this.waiting = new Promise<void>(resolve => { this.resolve = resolve; });
	}
	release(): void { this.blocked = false; this.resolve(); }
	async wait(): Promise<void> { this.reads += 1; await this.waiting; }
}

export class AttendanceLoadingFixture {
	identity = { memberID, companyID, isAdmin: true, email: 'member@example.com' };
	membershipReads = 0;
	calendarTitle = '제품 점검';
	includeCompanySummary = true;
	current = new LoadingResponseGate();
	teams = new LoadingResponseGate();
	members = new LoadingResponseGate();
	progress = new LoadingResponseGate();
	month = new LoadingResponseGate();
	calendar = new LoadingResponseGate();
	leave = new LoadingResponseGate();
	changes = new LoadingResponseGate();
	async answer(route: Route): Promise<void> {
		const tool = new URL(route.request().url()).pathname.split('/').at(-2);
		const posted: { input: Record<string, unknown> } = route.request().postDataJSON();
		const input = posted.input;
		let answer: unknown;
		let gate: LoadingResponseGate | undefined;
		if (tool === 'attendance_current_get') {
			gate = this.current;
			answer = { memberID, email: 'member@example.com', companyID, timeZone: 'Asia/Seoul', serverTime: loadingFixtureTime, backdatedAfterMinutes: 60, workLocations, authorization: { isAdmin: true, teamViewVisibleToAll: true }, todayEvents: [ownEvent], latestEvent: ownEvent, activeLeave: null };
		} else if (tool === 'attendance_team_dashboard_get' || tool === 'attendance_team_page_get') {
			gate = input.pageKind === 'members' ? this.members : this.teams;
			answer = { companyID, companyName: '샘플컴퍼니', companySummary: this.includeCompanySummary ? { memberCount: 48, working: 30, done: 6, away: 6, notStarted: 6, needsCheckout: 0 } : undefined, timeZone: 'Asia/Seoul', serverTime: loadingFixtureTime, authorization: { isAdmin: true, teamViewVisibleToAll: true }, teamOffset: 0, teamLimit: 6, teamTotal: 6, teams, selectedTeamKey: input.selectedTeamKey || null, memberOffset: 0, memberLimit: 24, memberTotal: 8, members: people.map(person => ({ memberID: person.personID, name: person.name, email: person.email, teamKey: String(input.selectedTeamKey || 'team-0'), status: 'working', latestAt: ownEvent.occurredAt, location: '사무실' })) };
		} else if (tool === 'attendance_list') {
			gate = input.to === '2026-10-06' ? this.progress : this.month;
			const personHints = input.personHints;
			const selectedPeople = Array.isArray(personHints) ? people.filter(person => personHints.includes(person.personID)) : people;
			const attendance = selectedPeople.map(person => ({ eventID: `${person.personID}-in`, personID: person.personID, person: person.name, kind: 'clock_in', date: '2026-10-06', time: '08:00', occurredAt: ownEvent.occurredAt, location: '사무실', wasCorrected: false, originalDate: null, originalTime: null, originalOccurredAt: null, reason: null }));
			answer = { from: input.from, to: input.to, serverTime: loadingFixtureTime, backdatedAfterMinutes: 60, count: attendance.length, attendance };
		} else if (tool === 'leave_list') {
			gate = this.leave;
			const leave = input.to && String(input.to) < '2026-10-15' ? [] : people.slice(0, input.scope === 'all' ? 3 : 1).map((person, index) => ({ leaveID: `sample-leave-${index}`, personID: person.personID, person: person.name, kindID: 'annual', kind: '연차', days: 1, status: 'requested', isPaid: true, isDeducted: true, startDate: '2026-10-15', endDate: '2026-10-15', startsAt: '2026-10-15T00:00:00+09:00', endsAt: '2026-10-16T00:00:00+09:00', note: '개인 일정' }));
			answer = { count: leave.length, leave, registeredKinds: ['annual'] };
		} else if (tool === 'attendance_leave_policy_get') answer = defaultLeavePolicy();
		else if (tool === 'leave_balance') answer = { scope: input.scope || 'mine', year: 2026, count: 8, balances: people.map(person => ({ personID: person.personID, personName: person.name, grantedDays: 15, remainingDays: 12, usedDays: 3, tracking: 'tracked' })) };
		else if (tool === 'attendance_changes_page_get') {
			gate = this.changes;
			const attendance = people.slice(0, 3).map((person, index) => ({ eventID: `sample-change-${index}`, personID: person.personID, person: person.name, personEmail: person.email, kind: 'clock_in', date: '2026-10-06', time: '08:00', originalDate: '2026-10-06', originalTime: '07:30', originalLocation: '사무실', location: '사무실', previousRecorded: true, reason: '시간 정정', teamName: '제품개발팀', changedBySource: 'observed', changedByName: person.name, changedByEmail: person.email }));
			answer = { attendance, totalCount: attendance.length };
		}
		else if (tool === 'person_list') answer = { requesterID: memberID, count: people.length, people };
		else if (tool === 'person_picture_list') answer = { count: 0, pictures: [] };
		else if (tool === 'company_settings_get') answer = { name: '샘플컴퍼니', locale: 'ko', timeZone: 'Asia/Seoul', currencyCode: 'KRW', workLocations, leaveDays: 15, teamViewVisibleToAll: true, profileImageURL: null };
		else if (tool === 'event_list') { gate = this.calendar; answer = { count: 1, events: [{ eventID: 'sample-event', title: this.calendarTitle, note: '', location: '', startsAt: '2026-10-06T01:00:00.000Z', endsAt: '2026-10-06T02:00:00.000Z', isWholeDay: false, participants: [], updatedAt: loadingFixtureTime, source: 'event', readOnly: false }] }; }
		else if (tool === 'company_holiday_list') answer = { count: 0, holidays: [] };
		else { await route.fulfill({ status: 503, json: { error: `No UI fixture for ${tool}` } }); return; }
		await gate?.wait();
		if (gate?.fail) { await route.fulfill({ status: 503, json: { error: '샘플 응답을 불러오지 못했습니다' } }); return; }
		await route.fulfill({ json: { result: answer } });
	}
	async install(page: Page): Promise<void> {
		await page.clock.setFixedTime(new Date(loadingFixtureTime));
		await page.emulateMedia({ colorScheme: 'light', reducedMotion: 'reduce' });
		if (!await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)) throw new Error('Reduced-motion fixture preference was not applied');
		await page.addInitScript(({ memberID, time }) => {
			const issuedAt = Date.parse(time) / 1000;
			const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
			localStorage.setItem('mode-watcher-mode', 'light');
			localStorage.setItem('sb-127-auth-token', JSON.stringify({ access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 36000 })}.fixture`, refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 36000, expires_at: issuedAt + 36000, user: { id: memberID, email: 'member@example.com', aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: time } }));
		}, { memberID, time: loadingFixtureTime });
		await page.route('http://127.0.0.1:56801/**', route => {
			const path = new URL(route.request().url()).pathname;
			if (path === '/rest/v1/member') { this.membershipReads += 1; return route.fulfill({ json: { id: this.identity.memberID, company_id: this.identity.companyID, is_admin: this.identity.isAdmin, name: '이샘플1', company: { slug: 'example-co', locale: 'ko' } } }); }
			if (path === '/auth/v1/user') return route.fulfill({ json: { id: this.identity.memberID, email: this.identity.email } });
			return route.fulfill({ json: [] });
		});
		await page.route('**/api/member/me', route => route.fulfill({ json: { member: { memberID } } }));
		await page.route('**/agent/api/**', route => route.fulfill({ json: {} }));
		await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
		await page.route('**/api/calendar/holidays?**', route => route.fulfill({ json: { holidays: [], degraded: false } }));
		await page.route('**/api/v1/tools/*/invoke', route => this.answer(route));
	}
}
