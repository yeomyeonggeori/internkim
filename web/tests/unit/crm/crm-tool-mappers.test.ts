import { describe, expect, test } from 'bun:test';
import {
	activityResponseOf,
	contactResponseOf,
	opportunityResponseOf,
	organizationResponseOf,
	type CRMActivityToolResult,
	type CRMContactToolResult,
	type CRMOpportunityToolResult,
	type CRMOrganizationToolResult
} from '../../../src/routes/crm/crm-tool-mappers';
import { CRMApiError } from '../../../src/routes/crm/crm-error';

const audit = {
	createdAt: '2026-09-01T00:00:00.000Z',
	createdByPersonID: 'member-one',
	updatedAt: '2026-09-04T00:00:00.000Z',
	updatedByPersonID: 'member-one',
	archivedAt: null,
	archivedByPersonID: ''
};

function organization(overrides: Partial<CRMOrganizationToolResult> = {}): CRMOrganizationToolResult {
	return {
		organizationID: 'organization-one',
		name: 'ABC상사',
		status: 'active',
		types: ['customer'],
		tags: [],
		importance: 'high',
		ownerPersonID: 'member-one',
		address: '',
		description: '',
		audit,
		...overrides
	};
}

function contact(overrides: Partial<CRMContactToolResult> = {}): CRMContactToolResult {
	return {
		contactID: 'contact-one',
		organizationID: 'organization-one',
		name: '박예시',
		email: 'yesi@example.com',
		phoneNumber: '',
		role: '구매팀장',
		department: '',
		description: '',
		audit,
		...overrides
	};
}

function opportunity(overrides: Partial<CRMOpportunityToolResult> = {}): CRMOpportunityToolResult {
	return {
		opportunityID: 'opportunity-one',
		organizationID: 'organization-one',
		contactID: 'contact-one',
		title: 'ABC상사 도입',
		business: '영업',
		pipeline: 'partnership',
		stage: 'review',
		stagePosition: 2,
		stageChangedAt: '2026-09-04T00:00:00.000Z',
		ownerPersonID: 'member-one',
		amountMinor: 18000000,
		currencyCode: 'KRW',
		baseAmountMinor: null,
		baseCurrencyCode: '',
		importance: 'high',
		expectedCloseAt: '2026-09-30T14:59:59.999Z',
		expectedCloseTimeZone: 'Asia/Seoul',
		lostReason: '',
		description: '',
		activityCount: 2,
		audit,
		...overrides
	};
}

function activity(overrides: Partial<CRMActivityToolResult> = {}): CRMActivityToolResult {
	return {
		activityID: 'activity-one',
		organizationID: 'organization-one',
		opportunityID: 'opportunity-one',
		contactID: '',
		business: '영업',
		kind: 'meeting',
		title: '킥오프 미팅',
		occurredAt: '2026-09-02T00:00:00.000Z',
		content: '요구사항을 들었다',
		taskStatus: 'completed',
		ownerPersonID: 'member-one',
		requesterPersonID: 'member-two',
		isEvent: false,
		isWholeDay: false,
		startsAt: '',
		endsAt: '',
		notifyMinutesBefore: null,
		location: '',
		createdAt: '2026-09-02T00:00:00.000Z',
		updatedAt: '2026-09-03T00:00:00.000Z',
		...overrides
	};
}

describe('what the CRM tools answer becomes what the screens read', () => {
	test('an organization keeps its owner and drops the empty fields', () => {
		const answered = organizationResponseOf(organization({ address: '서울' }));

		expect(answered.id).toBe('organization-one');
		expect(answered.address).toBe('서울');
		expect(answered.description).toBeUndefined();
		expect(answered.audit.archivedAt).toBeUndefined();
	});

	test('an unknown status or importance falls back rather than reaching a badge', () => {
		const answered = organizationResponseOf(organization({ status: 'invented', importance: 'urgent' }));

		expect(answered.status).toBe('active');
		expect(answered.importance).toBe('medium');
	});

	test('a contact keeps the email its hint resolves on', () => {
		expect(contactResponseOf(contact()).email).toBe('yesi@example.com');
		expect(contactResponseOf(contact({ email: '' })).email).toBeUndefined();
	});

	test('a deal keeps the moment it should close and the person it is with', () => {
		const answered = opportunityResponseOf(opportunity());

		expect(answered.dueAt).toBe('2026-09-30T14:59:59.999Z');
		expect(answered.dueTimeZone).toBe('Asia/Seoul');
		expect(answered.contacts).toEqual([{ contactID: 'contact-one' }]);
		expect(answered.baseCurrencyCode).toBeUndefined();
	});

	test('a stage the screens do not know is refused rather than drawn', () => {
		expect(() => opportunityResponseOf(opportunity({ stage: 'negotiating' }))).toThrow(CRMApiError);
	});

	test('an activity keeps when it happened and who recorded it', () => {
		const answered = activityResponseOf(activity());

		expect(answered.occurredAt).toBe('2026-09-02T00:00:00.000Z');
		expect(answered.taskOwnerID).toBe('member-one');
		expect(answered.audit.createdByPersonID).toBe('member-two');
		expect(answered.location).toBeUndefined();
	});

	test('an activity in the calendar keeps its hours and its reminder', () => {
		const answered = activityResponseOf(
			activity({
				isEvent: true,
				startsAt: '2026-09-02T00:00:00.000Z',
				endsAt: '2026-09-02T01:00:00.000Z',
				notifyMinutesBefore: 30,
				location: '회의실'
			})
		);

		expect(answered.isEvent).toBe(true);
		expect(answered.endsAt).toBe('2026-09-02T01:00:00.000Z');
		expect(answered.notifyMinutesBefore).toBe(30);
		expect(answered.location).toBe('회의실');
	});
});
