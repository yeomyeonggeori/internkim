import { describe, expect, test } from 'bun:test';
import { interimCurrencyCatalogue } from '../../../src/lib/currency/currency-catalogue';
import { buildCRMKPICards } from '../../../src/routes/crm/crm-kpi';
import type { CRMOrganization, CRMCurrency, CRMOpportunity, CRMOpportunityStage, CRMPipeline, CRMPipelineStage } from '../../../src/routes/crm/crm-types';
import type { CRMViewCurrencyReader } from '../../../src/routes/crm/crm-view-currency.svelte';
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
		business: '사업하나',
		name: id,
		pipeline: 'sales',
		stage,
		ownerName: '이샘플',
		ownerEmail: 'sample@example.com',
		expectedValue,
		currency,
		importance: 'medium',
		targetDate: '2026-08-01',
		staleDays
	};
}

const pipelines: CRMPipeline[] = [];

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

		const pipelineHealth = buildCRMKPICards(interimCurrencyCatalogue, [], opportunities, [], pipelines, stages, crmText.ko)[0];

		expect(pipelineHealth?.totalValue).toBe('₩1,700만');
		expect(pipelineHealth?.totalMoneyDetails).toEqual([
			{ currency: 'KRW', displayValue: '₩1,700만' },
			{ currency: 'USD', displayValue: '$2만' },
			{ currency: 'JPY', displayValue: '¥320만' },
			{ currency: 'EUR', displayValue: '€5만' }
		]);
		expect(pipelineHealth?.segments[0]).toMatchObject({
			label: '정상 진행',
			displayValue: '2건',
			tone: 'neutral',
			moneyDetails: [
				{ currency: 'KRW', displayValue: '₩1,200만' },
				{ currency: 'USD', displayValue: '$2만' }
			]
		});
		expect(pipelineHealth?.segments[1]).toMatchObject({
			label: '정체',
			displayValue: '€5만'
		});
		expect(pipelineHealth?.segments[1]?.moneyDetails).toBe(undefined);
		expect(pipelineHealth?.segments[2]).toMatchObject({
			label: '보류',
			displayValue: '2건',
			tone: 'attention',
			moneyDetails: [
				{ currency: 'KRW', displayValue: '₩500만' },
				{ currency: 'JPY', displayValue: '¥320만' }
			]
		});
	});

	test('derives the recent-contact segment from the supplied current date', () => {
		const organizations = [organization('recent', '2026-08-02'), organization('threshold', '2026-07-04'), organization('older', '2026-07-03')];

		const relationshipHealth = buildCRMKPICards(interimCurrencyCatalogue, organizations, [], [], [], [], crmText.ko, '2026-08-03')[1];

		expect(relationshipHealth?.segments[0]?.value).toBe(2);
		expect(relationshipHealth?.segments[1]?.value).toBe(1);
	});

	test('splits open progress by the pipelines the company defined, folding the rest into one', () => {
		const pipelines: CRMPipeline[] = [
			{ pipeline: 'e6c1', label: '연구 협력', direction: 'outbound', isActive: true },
			{ pipeline: 'a24f', label: '판매', direction: 'outbound', isActive: true }
		];
		const opportunities = [
			{ ...opportunity('research-one', 'KRW', 1000, 'in_progress', 1), pipeline: 'e6c1' },
			{ ...opportunity('research-two', 'KRW', 1000, 'in_progress', 1), pipeline: 'e6c1' },
			{ ...opportunity('sales-one', 'KRW', 1000, 'in_progress', 1), pipeline: 'a24f' },
			{ ...opportunity('grant-one', 'KRW', 1000, 'in_progress', 1), pipeline: 'b91d' },
			{ ...opportunity('grant-two', 'KRW', 1000, 'in_progress', 1), pipeline: 'c02e' },
			{ ...opportunity('grant-three', 'KRW', 1000, 'in_progress', 1), pipeline: 'd73a' }
		];
		const stages: CRMPipelineStage[] = [{ stage: 'in_progress', label: 'in_progress', position: 1, outcome: 'open' }];

		const composition = buildCRMKPICards(interimCurrencyCatalogue, [], opportunities, [], pipelines, stages, crmText.ko)[3];

		expect(composition?.totalValue).toBe('6');
		expect(composition?.segments.map((segment) => [segment.label, segment.value])).toEqual([
			['연구 협력', 2],
			['판매', 1],
			['b91d', 1],
			['기타', 2]
		]);
	});

	test('paints each composition segment with the colour its pipeline definition carries, leaving the remainder to the tone ladder', () => {
		const pipelines: CRMPipeline[] = [
			{ pipeline: 'e6c1', label: '연구 협력', direction: 'outbound', isActive: true, color: '#2563eb' },
			{ pipeline: 'a24f', label: '판매', direction: 'outbound', isActive: true, color: '#0d9488' },
			{ pipeline: 'b91d', label: '후원', direction: 'outbound', isActive: true }
		];
		const opportunities = [
			{ ...opportunity('research', 'KRW', 1000, 'in_progress', 1), pipeline: 'e6c1' },
			{ ...opportunity('sales', 'KRW', 1000, 'in_progress', 1), pipeline: 'a24f' },
			{ ...opportunity('sponsor', 'KRW', 1000, 'in_progress', 1), pipeline: 'b91d' },
			{ ...opportunity('other', 'KRW', 1000, 'in_progress', 1), pipeline: 'c02e' }
		];
		const stages: CRMPipelineStage[] = [{ stage: 'in_progress', label: 'in_progress', position: 1, outcome: 'open' }];

		const composition = buildCRMKPICards(interimCurrencyCatalogue, [], opportunities, [], pipelines, stages, crmText.ko)[3];

		expect(composition?.segments.map((segment) => [segment.label, segment.color])).toEqual([
			['연구 협력', '#2563eb'],
			['판매', '#0d9488'],
			['후원', undefined],
			['기타', undefined]
		]);
	});

	test('counts the same records in the breakdown as in the hero value beside it', () => {
		const organizations = [
			{ ...organization('active-recent', '2026-08-02'), status: 'active' as const },
			{ ...organization('paused-one', '2026-08-02'), status: 'paused' as const },
			{ ...organization('low-one', '2026-08-02'), importance: 'low' as const }
		];
		const opportunities = [
			{ ...opportunity('one', 'KRW', 1000, 'in_progress', 1), pipeline: 'e6c1' },
			{ ...opportunity('two', 'KRW', 1000, 'in_progress', 1), pipeline: 'f42b' }
		];
		const stages: CRMPipelineStage[] = [{ stage: 'in_progress', label: 'in_progress', position: 1, outcome: 'open' }];

		const cards = buildCRMKPICards(interimCurrencyCatalogue, organizations, opportunities, [], [], stages, crmText.ko, '2026-08-03');

		for (const card of [cards[1], cards[3]]) {
			const segmentTotal = card?.segments.reduce((sum, segment) => sum + segment.value, 0);
			expect(segmentTotal).toBe(Number(card?.totalValue));
		}
	});

	test('collapses multi-currency totals into a single estimate when a view currency is active', () => {
		const opportunities = [
			opportunity('moving-krw', 'KRW', 12000000, 'in_progress', 2),
			opportunity('moving-usd', 'USD', 20000, 'review', 5)
		];
		const stages: CRMPipelineStage[] = [
			{ stage: 'in_progress', label: 'in_progress', position: 1, outcome: 'open' },
			{ stage: 'review', label: 'review', position: 2, outcome: 'open' }
		];
		const view: CRMViewCurrencyReader = {
			selected: 'USD',
			viewAmount: (value, currency) =>
				currency === 'USD'
					? { value, currency: 'USD', isConverted: true }
					: { value: value * 0.00075, currency: 'USD', isConverted: true }
		};

		const pipelineHealth = buildCRMKPICards(interimCurrencyCatalogue, [], opportunities, [], pipelines, stages, crmText.ko, undefined, 'ko', view)[0];

		expect(pipelineHealth?.totalValue).toBe('$2.9만');
		expect(pipelineHealth?.totalMoneyDetails).toBe(undefined);
	});

	test('falls back to the view-currency total, never a currency count, when the rates do not cover every total', () => {
		const opportunities = [
			opportunity('moving-krw', 'KRW', 12000000, 'in_progress', 2),
			opportunity('moving-usd', 'USD', 20000, 'review', 5),
			opportunity('stalled-eur', 'EUR', 50000, 'in_progress', 15),
			opportunity('on-hold-jpy', 'JPY', 3200000, 'on_hold', 4)
		];
		const stages: CRMPipelineStage[] = [
			{ stage: 'in_progress', label: 'in_progress', position: 1, outcome: 'open' },
			{ stage: 'review', label: 'review', position: 2, outcome: 'open' },
			{ stage: 'on_hold', label: 'on_hold', position: 3, outcome: 'on_hold' }
		];
		const view: CRMViewCurrencyReader = {
			selected: 'USD',
			viewAmount: (value, currency) => (currency === 'USD' ? { value, currency: 'USD', isConverted: true } : { value, currency, isConverted: false })
		};

		const pipelineHealth = buildCRMKPICards(interimCurrencyCatalogue, [], opportunities, [], pipelines, stages, crmText.ko, undefined, 'ko', view)[0];

		expect(pipelineHealth?.totalValue).toBe('$2만');
		expect(pipelineHealth?.totalMoneyDetails).toEqual([
			{ currency: 'KRW', displayValue: '₩1,200만' },
			{ currency: 'USD', displayValue: '$2만' },
			{ currency: 'JPY', displayValue: '¥320만' },
			{ currency: 'EUR', displayValue: '€5만' }
		]);
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
