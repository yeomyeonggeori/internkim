import { afterEach, describe, expect, test } from 'bun:test';
import { CRMApiError, loadCRMData } from '../../../src/routes/crm/crm-api';

const originalFetch = globalThis.fetch;

afterEach(() => {
	globalThis.fetch = originalFetch;
});

describe('CRM API client', () => {
	test('loads records and pipeline stages from the service endpoints', async () => {
		const requestedPaths: string[] = [];
		setFetch(async (input) => {
			const path = String(input);
			requestedPaths.push(path);
			return Response.json(documentFor(path));
		});

		const data = await loadCRMData();

		expect(data.accounts).toHaveLength(1);
		expect(data.accounts[0]?.types).toEqual(['portfolio']);
		expect(data.opportunities[0]?.dueTimeZone).toBe('Asia/Seoul');
		expect(data.opportunities[0]?.contacts).toEqual([{ contactID: 'contact-1', isPrimary: true }]);
		expect(data.stages.map((stage) => stage.stage)).toEqual(['lead', 'qualified']);
		expect(requestedPaths).toContain('/crm/api/pipelines/sales/stages');
	});

	test('fails closed when a response does not match the CRM contract', async () => {
		setFetch(async (input) => Response.json(String(input).includes('/accounts') ? { accounts: [{ id: 42 }] } : documentFor(String(input))));

		await expect(loadCRMData()).rejects.toBeInstanceOf(CRMApiError);
	});

	test('preserves service error status and code', async () => {
		setFetch(async () => Response.json(
			{ error: { code: 'permission_denied', message: 'CRM access required' } },
			{ status: 403 }
		));

		try {
			await loadCRMData();
			throw new Error('expected loadCRMData to reject');
		} catch (error) {
			if (!(error instanceof CRMApiError)) throw error;
			expect(error.status).toBe(403);
			expect(error.code).toBe('permission_denied');
		}
	});
});

function setFetch(handler: (input: Parameters<typeof fetch>[0]) => Promise<Response>): void {
	const fetchMock: typeof fetch = Object.assign(handler, { preconnect: originalFetch.preconnect });
	globalThis.fetch = fetchMock;
}

function documentFor(path: string): object {
	const audit = {
		createdAt: '2026-08-03T00:00:00Z',
		createdByPersonID: 'person-owner',
		updatedAt: '2026-08-03T00:00:00Z',
		updatedByPersonID: 'person-owner'
	};
	if (path.endsWith('/accounts')) return { accounts: [{ id: 'account-1', name: '테스트 관계처', status: 'active', types: ['portfolio'], tags: [], importance: 'high', ownerPersonID: 'person-owner', audit }] };
	if (path.endsWith('/contacts')) return { contacts: [] };
	if (path.endsWith('/opportunities')) return { opportunities: [{ id: 'opportunity-1', accountID: 'account-1', name: '테스트 진행 건', pipeline: 'sales', stage: 'lead', stagePosition: 1024, stageChangedAt: '2026-08-03T00:00:00Z', ownerPersonID: 'person-owner', amountMinor: 1000, currencyCode: 'KRW', importance: 'high', dueAt: '2026-08-10T03:00:00Z', dueTimeZone: 'Asia/Seoul', contacts: [{ contactID: 'contact-1', isPrimary: true }], audit }] };
	if (path.endsWith('/activities')) return { activities: [] };
	if (path.endsWith('/pipelines')) return { pipelines: [{ pipeline: 'sales', label: '판매', direction: 'outbound', isActive: true }] };
	if (path.endsWith('/lost-reasons')) return { lostReasons: [] };
	if (path.endsWith('/pipelines/sales/stages')) return { stages: [{ pipeline: 'sales', stage: 'lead', position: 1, outcome: 'open' }, { pipeline: 'sales', stage: 'qualified', position: 2, outcome: 'open' }] };
	throw new Error(`unexpected CRM test path: ${path}`);
}
