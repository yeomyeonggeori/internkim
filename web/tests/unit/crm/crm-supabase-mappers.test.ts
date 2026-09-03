import { describe, expect, test } from 'bun:test';
import {
	crmDataResponseOf,
	crmVocabularyOf,
	type ContactRow,
	type OpportunityRow,
	type OrganizationRow
} from '../../../src/routes/crm/crm-supabase-mappers';

const audit = {
	created_at: '2026-08-19T01:00:00.000Z',
	created_by: '00000000-0000-0000-0000-000000000101',
	updated_at: '2026-08-19T02:00:00.000Z',
	updated_by: '00000000-0000-0000-0000-000000000101',
	archived_at: null,
	archived_by: null
};

describe('Supabase CRM mapper', () => {
	test('maps central organization, contact, opportunity and task rows into the CRM contract', () => {
		const organization: OrganizationRow = {
			...audit,
			id: '00000000-0000-0000-0000-000000000201',
			name: '샘플 협력 기관',
			status: 'active',
			types: ['partner'],
			tags: ['sample'],
			importance: 'high',
			owner_id: '00000000-0000-0000-0000-000000000101',
			address: '샘플 주소',
			description: '샘플 관계처'
		};
		const contact: ContactRow = {
			...audit,
			id: '00000000-0000-0000-0000-000000000301',
			organization_id: organization.id,
			name: '이샘플',
			email: 'sample@example.com',
			phone: null,
			title: '담당자',
			department: null,
			description: null
		};
		const opportunity: OpportunityRow = {
			...audit,
			id: '00000000-0000-0000-0000-000000000401',
			organization_id: organization.id,
			contact_id: contact.id,
			name: '샘플 협력 건',
			business: '샘플 사업',
			pipeline_id: 'partnership',
			stage_id: 'review',
			stage_position: 0,
			stage_changed_at: audit.updated_at,
			owner_id: audit.updated_by,
			amount_minor: 100000,
			currency_code: 'KRW',
			base_amount_minor: 100000,
			base_currency_code: 'KRW',
			importance: 'medium',
			due_at: null,
			due_time_zone: null,
			lost_reason: '일정 재조정이 어려워 무산',
			description: null
		};

		const data = crmDataResponseOf([organization], [contact], [opportunity], [{
			id: '00000000-0000-0000-0000-000000000501',
			organization_id: organization.id,
			opportunity_id: opportunity.id,
			contact_id: contact.id,
			title: '샘플 미팅',
			note: '협력 범위 확인',
			business: '샘플 사업',
			type: 'meeting',
			due_at: '2026-08-20T03:00:00.000Z',
			starts_at: null,
			ends_at: null,
			is_event: false,
			is_whole_day: false,
			notify_minutes_before: null,
			location: null,
			created_at: audit.created_at,
			updated_at: audit.updated_at,
			requester_id: audit.created_by,
			status: 'todo',
			task_participant: [{ member_id: audit.created_by }]
		}], {
			organization_types: [{ id: 'partner', name: '파트너' }],
			pipelines: [{ id: 'partnership', name: '파트너십' }]
		});

		expect(data.organizations[0]).toMatchObject({ id: organization.id, ownerPersonID: audit.updated_by });
		expect(data.contacts[0]).toMatchObject({ organizationID: organization.id });
		expect(data.opportunities[0]).toMatchObject({
			organizationID: organization.id,
			contacts: [{ contactID: contact.id }],
			lostReason: '일정 재조정이 어려워 무산'
		});
		expect(data.activities[0]).toMatchObject({
			organizationID: organization.id,
			opportunityID: opportunity.id,
			contactID: contact.id,
			occurredAt: '2026-08-20T03:00:00.000Z'
		});
		expect(data.pipelines).toEqual([{ pipeline: 'partnership', label: '파트너십', direction: '', isActive: true }]);
	});

	test('filters archived organizations, contacts and opportunities', () => {
		const archived = { ...audit, archived_at: '2026-08-20T00:00:00.000Z', archived_by: audit.updated_by };
		const data = crmDataResponseOf(
			[{ ...archived, id: 'organization', name: '보관 기관', status: 'active', types: [], tags: [], importance: 'medium', owner_id: null, address: null, description: null }],
			[{ ...archived, id: 'contact', organization_id: null, name: '보관 연락처', email: null, phone: null, title: null, department: null, description: null }],
			[{ ...archived, id: 'opportunity', organization_id: 'organization', contact_id: null, name: '보관 진행 건', business: null, pipeline_id: 'pipeline', stage_id: 'waiting', stage_position: 0, stage_changed_at: audit.updated_at, owner_id: null, amount_minor: null, currency_code: null, base_amount_minor: null, base_currency_code: null, importance: 'medium', due_at: null, due_time_zone: null, lost_reason: null, description: null }],
			[],
			{}
		);
		expect(data.organizations).toEqual([]);
		expect(data.contacts).toEqual([]);
		expect(data.opportunities).toEqual([]);
	});

	test('rejects malformed vocabulary entries without inventing definitions', () => {
		expect(crmVocabularyOf({
			organization_types: [{ id: 'partner' }],
			pipelines: [{ id: 'sales', name: '판매' }]
		})).toEqual({
			organization_types: [],
			pipelines: [{ id: 'sales', name: '판매', direction: undefined }]
		});
	});
});
