import { describe, expect, test } from 'bun:test';
import { interimCurrencyCatalogue } from '../../../src/lib/currency/currency-catalogue';
import {
	activityPayload,
	activityPayloadFromDraft,
	organizationPayloadFromDraft,
	contactPayload,
	CRMOwnerResolutionError,
	localDateToUTC,
	mapCRMViewData,
	opportunityPayload,
	opportunityPayloadFromDraft,
	resolveOwner,
	utcToLocalDate
} from '../../../src/routes/crm/crm-mappers';
import type { CRMDataResponse } from '../../../src/routes/crm/crm-api-types';

describe('CRM service mappers', () => {
	test('round-trips local due dates through UTC in different time zones', () => {
		for (const [date, timeZone] of [
			['2026-08-10', 'Asia/Seoul'],
			['2026-03-08', 'America/New_York'],
			['2026-10-25', 'Europe/London']
		] as const) {
			expect(utcToLocalDate(localDateToUTC(date, timeZone), timeZone)).toBe(date);
		}
	});

	test('stores date-only due dates at the end of the selected local day', () => {
		expect(localDateToUTC('2026-08-10', 'Asia/Seoul')).toBe('2026-08-10T14:59:59.000Z');
		expect(localDateToUTC('2026-08-10', 'America/New_York')).toBe('2026-08-11T03:59:59.000Z');
		expect(localDateToUTC('2026-03-08', 'America/New_York')).toBe('2026-03-09T03:59:59.000Z');
	});

	test('displays a due date in its stored time zone regardless of viewer time zone', () => {
		const data = serviceData();
		data.opportunities[0] = {
			...data.opportunities[0]!,
			dueAt: '2026-08-11T03:59:59.000Z',
			dueTimeZone: 'America/New_York'
		};

		expect(mapCRMViewData(data, [], interimCurrencyCatalogue, 'Asia/Seoul').opportunities[0]?.targetDate).toBe('2026-08-10');
	});

	test('preserves empty and multiple relationship types in create payloads', () => {
		const owner = { userID: 'person-owner', handle: 'owner', name: '담당자', email: 'owner@example.com' };
		const draft = {
			kind: 'relationship' as const,
			name: '관계처',
			types: [] as const,
			status: 'prospect' as const,
			importance: 'medium' as const,
			ownerPersonID: 'person-owner',
			address: '',
			tags: [],
			description: ''
		};

		expect(organizationPayloadFromDraft({ ...draft, types: [] }, owner).types).toEqual([]);
		expect(organizationPayloadFromDraft({ ...draft, types: ['partner', 'portfolio'] }, owner).types).toEqual(['partner', 'portfolio']);
	});

	test('derives display names and organization metrics from service records', () => {
		const view = mapCRMViewData(
			serviceData(),
			[{ userID: 'person-owner', handle: 'owner', name: '담당자', email: 'owner@example.com' }],
			interimCurrencyCatalogue,
			'Asia/Seoul'
		);

		expect(view.organizations[0]?.ownerName).toBe('담당자');
		expect(view.organizations[0]?.openOpportunityCount).toBe(1);
		expect(view.organizations[0]?.expectedValues).toEqual({ KRW: 5000 });
		expect(view.organizations[0]?.lastContactDate).toBe('2026-08-04');
		expect(view.opportunities[0]?.targetDate).toBe('2026-08-10');
	});

	test('preserves realized base amounts for later terminal transitions', () => {
		const data = serviceData();
		data.opportunities[0] = {
			...data.opportunities[0]!,
			stage: 'done',
			baseAmountMinor: 4500,
			baseCurrencyCode: 'USD'
		};
		data.stages = [{ stage: 'done', label: 'done', position: 1, outcome: 'won' }];
		const opportunity = mapCRMViewData(data, [], interimCurrencyCatalogue, 'Asia/Seoul').opportunities[0];

		expect(opportunity?.baseAmountMinor).toBe(4500);
		expect(opportunity?.baseCurrencyCode).toBe('USD');
	});

	test('preserves contact and opportunity relationships in update payloads', () => {
		const view = mapCRMViewData(serviceData(), [], interimCurrencyCatalogue, 'Asia/Seoul');

		expect(contactPayload(view.contacts[0]!)).toMatchObject({ department: '파트너십' });
		expect(opportunityPayload(view.opportunities[0]!, interimCurrencyCatalogue)).toMatchObject({
			contacts: [{ contactID: 'contact-1' }]
		});
	});

	test('preserves stage-change kinds for updates while excluding them from create payloads', () => {
		const activity = mapCRMViewData(serviceData(), [], interimCurrencyCatalogue, 'Asia/Seoul').activities[0]!;
		activity.kind = 'stage_change';

		expect(activityPayload(activity).kind).toBe('stage_change');
		expect(activityPayloadFromDraft({
			kind: 'activity', organizationID: 'organization-1', opportunityID: 'opportunity-1', business: 'general',
			activityKind: 'stage_change', title: '직접 생성 불가', occurredAt: '2026-08-03T15:30', summary: '',
			taskOwnerID: '', taskStatus: '', calendar: { isRequested: false, isAllDay: true, startTime: '', endTime: '', location: '' }
		}).kind).toBe('event');
	});

	test('stores the selected internal owner team on opportunity creation', () => {
		const owner = { userID: 'person-ops', handle: 'ops', name: '운영 담당자', email: 'ops@example.com', groupID: 'team-ops' };
		const payload = opportunityPayloadFromDraft({
			kind: 'progress',
			organizationID: 'organization-1',
			contacts: [],
			business: 'general',
			name: '신규 진행 건',
			progressKind: 'sales',
			stage: 'waiting',
			lostReason: '',
			ownerPersonID: 'person-ops',
			currency: 'KRW',
			importance: 'medium',
			targetDate: '',
			description: '',
			calendar: { isRequested: false, isAllDay: true, startTime: '', endTime: '', location: '' }
		}, owner, 'Asia/Seoul', interimCurrencyCatalogue);

		expect(payload).toMatchObject({ ownerPersonID: 'person-ops', ownerCircleID: 'team-ops' });
	});

	test('rejects a non-empty owner hint that does not match the directory', () => {
		const people = [{ userID: 'person-owner', handle: 'owner', name: '담당자', email: 'owner@example.com' }];

		try {
			resolveOwner(people, 'unknown@example.com', 'owner@example.com');
			throw new Error('expected resolveOwner to reject');
		} catch (error) {
			if (!(error instanceof CRMOwnerResolutionError)) throw error;
			expect(error.code).toBe('owner_not_found');
		}
	});

	test('derives activity dates in the selected display time zone', () => {
		const people = [{ userID: 'person-owner', handle: 'owner', name: '담당자', email: 'owner@example.com' }];

		expect(mapCRMViewData(serviceData(), people, interimCurrencyCatalogue, 'Asia/Seoul').organizations[0]?.lastContactDate).toBe('2026-08-04');
		expect(mapCRMViewData(serviceData(), people, interimCurrencyCatalogue, 'America/New_York').organizations[0]?.lastContactDate).toBe('2026-08-03');
	});
});

function serviceData(): CRMDataResponse {
	const audit = {
		createdAt: '2026-08-01T00:00:00Z',
		createdByPersonID: 'person-owner',
		updatedAt: '2026-08-01T00:00:00Z',
		updatedByPersonID: 'person-owner'
	};
	return {
		organizations: [{ id: 'organization-1', name: '관계처', status: 'active', types: ['customer'], tags: [], importance: 'high', ownerPersonID: 'person-owner', audit }],
		contacts: [{ id: 'contact-1', organizationID: 'organization-1', name: '담당 연락처', department: '파트너십', ownerPersonID: 'person-owner', audit }],
		opportunities: [{ id: 'opportunity-1', organizationID: 'organization-1', name: '진행 건', pipeline: 'sales', stage: 'waiting', stagePosition: 1024, stageChangedAt: '2026-08-02T00:00:00Z', ownerPersonID: 'person-owner', amountMinor: 5000, currencyCode: 'KRW', importance: 'high', dueAt: '2026-08-10T03:00:00Z', dueTimeZone: 'Asia/Seoul', contacts: [{ contactID: 'contact-1' }], audit }],
		activities: [{ id: 'activity-1', organizationID: 'organization-1', kind: 'meeting', title: '미팅', occurredAt: '2026-08-03T15:30:00Z', content: '논의', audit }],
		pipelines: [{ pipeline: 'sales', label: '판매', direction: 'outbound', isActive: true }],
		stages: [{ stage: 'waiting', label: 'waiting', position: 1, outcome: 'open' }],
		lostReasons: [],
		vocabulary: { organization_types: [{ id: 'customer', name: '고객사' }], pipelines: [], stages: [], lost_reasons: [] },
		taskVocabulary: { businesses: [{ name: 'general' }], types: [{ name: 'meeting' }] }
	};
}
