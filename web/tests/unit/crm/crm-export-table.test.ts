import { describe, expect, test } from 'bun:test';
import { csvDocument } from '../../../src/lib/csv';
import { interimCurrencyCatalogue } from '../../../src/lib/currency/currency-catalogue';
import {
	crmExportFilename,
	crmExportRowMatches,
	crmExportTable,
	type CRMExportSource
} from '../../../src/routes/crm/crm-export-table';
import type { CRMActivity, CRMContact, CRMOpportunity, CRMOrganization } from '../../../src/routes/crm/crm-types';
import { crmText } from '../../../src/routes/crm/text';

const organization: CRMOrganization = {
	id: 'organization-1',
	name: '샘플상사',
	types: ['customer', 'partner'],
	status: 'active',
	importance: 'high',
	ownerName: '이샘플',
	ownerEmail: 'sample@example.com',
	team: '',
	address: '서울시 예시구 예시로 1',
	tags: ['제조', 'B2B'],
	description: '연말 "재계약", 논의\n두 번째 줄',
	lastContactDate: '',
	nextActionDate: '',
	openOpportunityCount: 1,
	expectedValues: {}
};

const contact: CRMContact = {
	id: 'contact-1',
	organizationID: 'organization-1',
	name: '박예시',
	title: '팀장',
	email: 'park@example.com',
	phone: '010-0000-0001'
};

const opportunity: CRMOpportunity = {
	id: 'opportunity-1',
	organizationID: 'organization-1',
	business: '제조',
	name: '해외 법인 도입',
	stage: 'in_progress',
	ownerName: '이샘플',
	ownerEmail: 'sample@example.com',
	expectedValue: 85000,
	currency: 'USD',
	baseAmountMinor: 116450000,
	baseCurrencyCode: 'KRW',
	importance: 'medium',
	targetDate: '2026-11-30',
	staleDays: 0,
	contacts: [{ contactID: 'contact-1' }]
};

const activity: CRMActivity = {
	id: 'activity-1',
	organizationID: 'organization-1',
	contactID: 'contact-1',
	opportunityID: 'opportunity-1',
	business: '제조',
	kind: 'meeting',
	title: '계약 조건 미팅',
	occurredAt: '2026-10-08T05:00:00.000Z',
	summary: '',
	taskID: 'task-1',
	taskStatus: 'completed',
	size: '',
	participantIDs: ['member-1', 'member-2'],
	participantNames: ['이샘플', '최견본']
};

function sourceWith(overrides: Partial<CRMExportSource> = {}): CRMExportSource {
	return {
		organizations: [organization, { ...organization, id: 'organization-2', name: '견본물류' }],
		contacts: [contact],
		opportunities: [opportunity],
		activities: [activity],
		organizationTypeDefinitions: [],
		pipelines: [],
		stages: [],
		currencyCatalogue: interimCurrencyCatalogue,
		companyBaseCurrency: 'KRW',
		timeZone: 'Asia/Seoul',
		text: crmText.ko,
		...overrides
	};
}

function cellOf(table: ReturnType<typeof crmExportTable>, rowIndex: number, column: string): string {
	return table.rows[rowIndex]?.cells[table.columns.indexOf(column)] ?? '';
}

describe('crmExportTable', () => {
	test('names each organization row after the organization it belongs to', () => {
		const table = crmExportTable('relationships', sourceWith());
		expect(table.rows.map((row) => row.organizationID)).toEqual(['organization-1', 'organization-2']);
		expect(table.rows[0]?.cells).toHaveLength(table.columns.length);
		expect(cellOf(table, 0, crmText.ko.tags)).toBe('제조, B2B');
		expect(cellOf(table, 0, crmText.ko.importance)).toBe(crmText.ko.importanceLabels.high);
	});

	test('writes a deal amount as plain numbers in its own currency and in the base currency', () => {
		const table = crmExportTable('pipeline', sourceWith());
		expect(cellOf(table, 0, crmText.ko.amount)).toBe('85000');
		expect(cellOf(table, 0, crmText.ko.currency)).toBe('USD');
		expect(cellOf(table, 0, crmText.ko.export.baseAmount.replace('{currency}', 'KRW'))).toBe('116450000');
		expect(cellOf(table, 0, crmText.ko.externalContact)).toBe('박예시');
	});

	test('lists every participant of an activity in one cell, at local time', () => {
		const table = crmExportTable('activities', sourceWith());
		expect(cellOf(table, 0, crmText.ko.assignee)).toBe('이샘플, 최견본');
		expect(cellOf(table, 0, crmText.ko.occurredAt)).toBe('2026-10-08 14:00');
		expect(cellOf(table, 0, crmText.ko.relatedProgress)).toBe('해외 법인 도입');
		expect(cellOf(table, 0, crmText.ko.registerCalendar)).toBe('');
	});

	test('ties a contact row to its organization', () => {
		const table = crmExportTable('contacts', sourceWith());
		expect(table.rows[0]?.organizationID).toBe('organization-1');
		expect(cellOf(table, 0, crmText.ko.organizationName)).toBe('샘플상사');
	});
});

describe('crmExportRowMatches', () => {
	test('matches a query against any cell, ignoring case', () => {
		const row = { id: 'one', organizationID: 'organization-1', cells: ['Sample Trading', 'sample@example.com'] };
		expect(crmExportRowMatches(row, '  SAMPLE@ ')).toBe(true);
		expect(crmExportRowMatches(row, 'missing')).toBe(false);
		expect(crmExportRowMatches(row, '')).toBe(true);
	});
});

describe('crm export file', () => {
	test('keeps quotes, commas and line breaks in a cell intact', () => {
		const table = crmExportTable('relationships', sourceWith());
		const document = csvDocument([table.columns, ...table.rows.map((row) => row.cells)]);
		expect(document.startsWith('﻿')).toBe(true);
		expect(document).toContain('"연말 ""재계약"", 논의\n두 번째 줄"');
		expect(document.endsWith('\r\n')).toBe(true);
	});

	test('names the file after the tab and the date', () => {
		expect(crmExportFilename('관계처', '2026-10-13')).toBe('CRM-관계처-2026-10-13.csv');
	});
});
