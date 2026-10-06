import type { Page } from '@playwright/test';
import { createDevMailMockState } from '../../dev-mail-mock-state';
import { createDevMemoryMockResponse, createDevMemoryMockState } from '../../dev-memory-mock-plugin';
import { createDevTasksMockResponse, createDevTasksMockState } from '../../dev-tasks-mock-plugin';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const email = 'member@example.com';
const date = '2026-10-06T03:00:00Z';
const people = Array.from({ length: 8 }, (_, index) => ({ personID: index ? `person-${index}` : memberID, name: ['이샘플', '박예시', '최견본'][index % 3], email: `member${index}@example.com`, isAdmin: index === 0, jobTitle: index % 2 ? '프로덕트 디자이너' : '소프트웨어 엔지니어', teamID: 'team-product', hireDate: '2024-01-01' }));
const audit = { createdAt: date, createdByPersonID: memberID, updatedAt: date, updatedByPersonID: memberID, archivedAt: null, archivedByPersonID: '' };
const organizations = Array.from({ length: 6 }, (_, index) => ({ organizationID: `organization-${index}`, name: `예시 파트너 ${index + 1}`, status: 'active', types: [], tags: [], importance: 'medium', ownerPersonID: memberID, address: '', description: '제품 협력과 고객 경험을 함께 준비합니다.', audit }));
const files = Array.from({ length: 7 }, (_, index) => ({ name: `${['주간-회고', '프로젝트-계획', '제품-요구사항'][index % 3]}-${index + 1}.md`, agentPath: `/workspace/private/sample/document-${index + 1}.md`, isDirectory: false, size: 2048 * (index + 1), modifiedAt: date }));

export class WorkspaceLoadingFixture {
	private gates = new Map<string, Promise<void>>();
	private releases = new Map<string, () => void>();
	readonly requested = new Set<string>();
	readonly refused = new Set<string>();
	omitMailPreview = false;
	readonly mail = createDevMailMockState(email);
	readonly memory = createDevMemoryMockState(email);
	readonly runs = createDevTasksMockState(email);

	hold(name: string): void {
		this.requested.delete(name);
		this.gates.set(name, new Promise<void>((resolve) => this.releases.set(name, resolve)));
	}

	release(name: string): void {
		this.releases.get(name)?.();
		this.gates.delete(name);
		this.releases.delete(name);
	}

	async waitFor(name: string): Promise<void> {
		this.requested.add(name);
		await this.gates.get(name);
	}

	tool(name: string, input: Record<string, unknown>): unknown {
		if (name === 'person_list') return { requesterID: memberID, people, count: people.length };
		if (name === 'team_list') return { count: 1, teams: [{ teamID: 'team-product', name: '제품팀', parentTeamID: '', position: 0 }] };
		if (name === 'company_settings_get') return { currencyCode: 'KRW', timeZone: 'Asia/Seoul', name: '예시 회사' };
		if (name === 'circle_list') return { circles: [] };
		if (name === 'crm_organization_list') return { organizations };
		if (name === 'crm_contact_list') return { contacts: [] };
		if (name === 'crm_opportunity_list') return { opportunities: organizations.slice(0, 3).map((organization, index) => ({ opportunityID: `opportunity-${index}`, organizationID: organization.organizationID, contactID: '', title: `협력 제안 ${index + 1}`, business: '', pipeline: 'sales', stage: 'in_progress', stagePosition: index, stageChangedAt: date, ownerPersonID: memberID, amountMinor: 1500000, currencyCode: 'KRW', baseAmountMinor: null, baseCurrencyCode: '', importance: 'medium', expectedCloseAt: '', expectedCloseTimeZone: '', lostReason: '', description: '', activityCount: 0, audit })) };
		if (name === 'crm_activity_list') return { activities: [], registeredLabels: { businesses: [], types: [] } };
		if (name === 'crm_vocabulary_get') return { organizationTypes: [], pipelines: [{ id: 'sales', name: '영업', color: 'blue' }] };
		if (name === 'mail_mailbox_list') return { mailboxes: this.mail.mailboxes };
		if (name === 'mail_message_list' || name === 'mail_message_search') return { messages: this.mail.messages.filter(message => message.mailbox === (input.mailbox || 'INBOX')).map(({ body, ...message }) => ({ ...message, preview: this.omitMailPreview ? '' : message.preview, isRead: true })), nextCursor: '' };
		if (name === 'mail_message_read') return this.mail.messages.find(message => message.uid === Number(input.uid));
		if (name === 'notification_settings_get') return { categories: [], mutedConversationIDs: [] };
		return {};
	}

	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		if (this.refused.has(capability)) return { status: 503, body: { error: 'Fixture temporary read failure' } };
		if (capability === 'person.files.roots') return { status: 200, body: { roots: [{ id: 'personal', label: '개인', agentPath: '/workspace/private/sample', kind: 'personal' }] } };
		if (capability === 'person.files.list') return { status: 200, body: { entries: files } };
		const paths: Record<string, string> = { 'person.memory.facts': '/memory/api/facts', 'person.memory.schedules': '/memory/api/schedules', 'person.runs.list': '/runs/api', 'person.runs.detail': '/runs/api/detail' };
		const pathname = paths[capability];
		if (!pathname) return { status: 200, body: {} };
		const searchParams = new URLSearchParams(Object.entries(body).map(([key, value]) => [key, String(value)]));
		const request = { method: 'GET', pathname, searchParams, body: '' };
		return (pathname.startsWith('/memory') ? createDevMemoryMockResponse(this.memory, request) : createDevTasksMockResponse(this.runs, request)) ?? { status: 404, body: {} };
	}
}

export async function prepareWorkspaceLoading(page: Page, fixture: WorkspaceLoadingFixture): Promise<void> {
	await page.clock.setFixedTime(new Date(date));
	await page.addInitScript(({ memberID, email }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({ access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`, refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600, user: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } }));
	}, { memberID, email });
	await page.route('http://127.0.0.1:56801/**', async route => {
		const pathname = new URL(route.request().url()).pathname;
		if (pathname === '/rest/v1/member') return route.fulfill({ json: { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } });
		await route.fulfill({ json: [] });
	});
	await page.route('**/api/member/me', route => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/auth/session**', route => route.fulfill({ json: { authenticated: true, email, isAdmin: true } }));
	await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/agent/api/**', route => route.fulfill({ json: {} }));
	await page.route('**/api/currencies', route => route.fulfill({ json: { currencies: [{ code: 'KRW', name: 'Korean Won', minorUnitDigits: 0 }] } }));
	await page.route('**/api/member/mail-account', async route => { await fixture.waitFor('mail-account'); await route.fulfill({ json: { account: fixture.mail.account } }); });
	await page.route('**/api/v1/tools/*/invoke', async route => {
		const name = new URL(route.request().url()).pathname.split('/').at(-2) ?? '';
		await fixture.waitFor(name);
		const requested: { input?: Record<string, unknown> } = route.request().postDataJSON();
		await route.fulfill(fixture.refused.has(name) ? { status: 503, json: { error: 'Fixture temporary read failure' } } : { json: { result: fixture.tool(name, requested.input ?? {}) } });
	});
	await page.routeWebSocket('**/company/*/client', socket => {
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: true }));
		socket.onMessage(async raw => {
			const message: { kind: string; capability: string; requestID: string; body?: Record<string, unknown> } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			await fixture.waitFor(message.capability);
			socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, ...fixture.host(message.capability, message.body ?? {}) }));
		});
	});
}
