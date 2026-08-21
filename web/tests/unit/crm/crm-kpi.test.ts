import { describe, expect, test } from 'bun:test';
import { interimCurrencyCatalogue } from '../../../src/lib/currency/currency-catalogue';
import { buildCRMKPICards } from '../../../src/routes/crm/crm-kpi';
import type { CRMOrganization, CRMCurrency, CRMOpportunity, CRMOpportunityStage, CRMPipelineStage } from '../../../src/routes/crm/crm-types';
import { crmText } from '../../../src/routes/crm/text';

function opportunity(
	id: string,
	currency: CRMCurrency,
	expectedValue: number,
	stage: CRMOpportunityStage,
	staleDays: number
): CRMOpportunity {
	return {
		id,
		organizationID: 'organization',
		business: '여명거리',
		name: id,
		pipeline: 'sales',
		stage,
		ownerName: '김테스트04',
		expectedValue,
		currency,
		importance: 'medium',
		targetDate: '2026-08-01',
		staleDays
	};
}

describe('CRM KPI money details', () => {
	test('keeps compact summaries while exposing every currency total', () => {
		const opportunities = [
			opportunity('moving-krw', 'KRW', 12000000, 'in_progress', 2),
			opportunity('moving-usd', 'USD', 20000, 'review', 5),
			opportunity('stalled-eur', 'EUR', 50000, 'in_progress', 15),
			opportunity('on-hold-krw', 'KRW', 5000000, 'on_hold', 3),
			opportunity('on-hold-jpy', 'JPY', 3200000, 'on_hold', 4)
		];
		const stages: CRMPipelineStage[] = [
			{ stage: 'in_progress', label: 'in_progress', position: 1, outcome: 'open' },
			{ stage: 'review', label: 'review', position: 2, outcome: 'open' },
			{ stage: 'on_hold', label: 'on_hold', position: 3, outcome: 'on_hold' }
		];

		const pipelineHealth = buildCRMKPICards(interimCurrencyCatalogue, [], opportunities, [], stages, crmText.ko)[0];

		expect(pipelineHealth?.totalValue).toBe('4개 통화');
		expect(pipelineHealth?.totalMoneyDetails).toEqual([
			{ currency: 'KRW', displayValue: '₩1,700만' },
			{ currency: 'USD', displayValue: '$20K' },
			{ currency: 'JPY', displayValue: '¥320만' },
			{ currency: 'EUR', displayValue: '€50K' }
		]);
		expect(pipelineHealth?.segments[0]).toMatchObject({
			label: '정상 진행',
			displayValue: '2건',
			moneyDetails: [
				{ currency: 'KRW', displayValue: '₩1,200만' },
				{ currency: 'USD', displayValue: '$20K' }
			]
		});
		expect(pipelineHealth?.segments[1]).toMatchObject({
			label: '정체',
			displayValue: '€50K'
		});
		expect(pipelineHealth?.segments[1]?.moneyDetails).toBe(undefined);
		expect(pipelineHealth?.segments[2]).toMatchObject({
			label: '보류',
			displayValue: '2건',
			moneyDetails: [
				{ currency: 'KRW', displayValue: '₩500만' },
				{ currency: 'JPY', displayValue: '¥320만' }
			]
		});
	});

	test('derives the recent-contact segment from the supplied current date', () => {
		const organizations = [organization('recent', '2026-08-02'), organization('threshold', '2026-07-04'), organization('older', '2026-07-03')];

		const relationshipHealth = buildCRMKPICards(interimCurrencyCatalogue, organizations, [], [], [], crmText.ko, '2026-08-03')[1];

		expect(relationshipHealth?.segments[0]?.value).toBe(2);
		expect(relationshipHealth?.segments[1]?.value).toBe(1);
	});
});

function organization(id: string, lastContactDate: string): CRMOrganization {
	return {
		id,
		name: id,
		types: ['customer'],
		status: 'active',
		importance: 'medium',
		ownerName: '담당자',
		ownerEmail: 'owner@example.com',
		team: '',
		tags: [],
		description: '',
		lastContactDate,
		nextActionDate: '',
		openOpportunityCount: 0,
		expectedValues: {}
	};
}
