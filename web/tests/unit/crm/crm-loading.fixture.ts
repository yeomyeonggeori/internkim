import { beforeEach, expect, mock, test } from 'bun:test';
import type { CRMDataResponse } from '../../../src/routes/crm/crm-api-types';
import type { CRMReadPart } from '../../../src/routes/crm/crm-public-api';
import type { UserRecord } from '../../../src/lib/organization/types';
import { crmText } from '../../../src/routes/crm/text';
import * as currency from '../../../src/lib/currency/currency-catalogue';
import { CRMApiError } from '../../../src/routes/crm/crm-error';

Object.assign(globalThis, { $state: <Value>(value: Value): Value => value });

type Directory = { records: UserRecord[]; availableGroups: [] };
const owner = { memberID: 'owner', handle: 'sample', email: 'owner@example.com', name: 'Sample Owner' };
let directory: () => Promise<Directory>;
let read: (previous?: CRMDataResponse, changed?: readonly CRMReadPart[]) => Promise<CRMDataResponse>;
let write: (name: string) => Promise<unknown>;
let catalogue: () => Promise<currency.CurrencyCatalogue>;
let baseCurrency: () => Promise<string>;

mock.module('../../../src/lib/currency/currency-catalogue', () => ({ ...currency, loadCurrencyCatalogue: () => catalogue() }));
mock.module('../../../src/lib/company/base-currency', () => ({ interimCompanyBaseCurrency: 'KRW', loadCompanyBaseCurrency: () => baseCurrency() }));
mock.module('../../../src/routes/crm/dev-crm-fixture', () => ({
	crmFixtureMode: false, crmFixtureActivityKindColors: {}, crmFixtureOrganizationTypeColors: {},
	crmFixturePipelineColors: {}, crmFixturePeople: [], crmOrganizations: [], crmActivities: [], crmContacts: [], crmOpportunities: []
}));
mock.module('../../../src/routes/crm/crm-data-source', () => ({
	CRMApiError, loadCRMOrganizationDirectory: () => directory(),
	loadCRMData: (previous?: CRMDataResponse, changed?: readonly CRMReadPart[]) => read(previous, changed),
	createCRMOrganization: () => write('organization'), createCRMContact: () => write('contact'),
	createCRMOpportunity: () => write('opportunity'), createCRMActivity: () => write('activity'),
	updateCRMOrganization: () => write('organization'), updateCRMContact: () => write('contact'),
	updateCRMOpportunity: () => write('opportunity'), updateCRMActivity: () => write('activity'),
	archiveCRMOrganization: () => write('organization'), archiveCRMOpportunity: () => write('opportunity'),
	saveCRMVocabulary: () => write('vocabulary'), positionCRMOpportunity: () => write('opportunity'),
	transitionCRMOpportunity: () => write('opportunity')
}));

const { CRMPageController } = await import('../../../src/routes/crm/crm-page-controller.svelte');

function snapshot(name = 'Sample Account'): CRMDataResponse {
	const audit = { createdAt: '2026-10-01', createdByPersonID: 'owner', updatedAt: '2026-10-01', updatedByPersonID: 'owner' };
	return {
		organizations: [{ id: 'organization', name, status: 'active', types: [], tags: [], importance: 'medium', ownerPersonID: 'owner', audit }],
		contacts: [{ id: 'contact', organizationID: 'organization', name: 'Sample Contact', ownerPersonID: 'owner', audit }],
		opportunities: [{ id: 'deal', organizationID: 'organization', name: 'Sample Deal', pipeline: 'sales', stage: 'waiting',
			stagePosition: 1, stageChangedAt: '2026-10-01', ownerPersonID: 'owner', amountMinor: 12345, currencyCode: 'BHD', importance: 'medium', audit }],
		activities: [], pipelines: [], vocabulary: { organization_types: [], pipelines: [] }, taskVocabulary: {}
	};
}

beforeEach(() => {
	directory = async () => ({ records: [owner], availableGroups: [] });
	read = async () => snapshot();
	write = async () => ({});
	catalogue = async () => [{ code: 'BHD', name: 'Bahraini Dinar', minorUnitDigits: 3 }];
	baseCurrency = async () => 'BHD';
});

test('slow directory does not delay complete amounts and metrics; contact edits remain available', async () => {
	const pending = Promise.withResolvers<Directory>();
	directory = () => pending.promise;
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	expect(controller.isLoading).toBe(false);
	expect(controller.hasData).toBe(true);
	expect(controller.isDirectoryReady).toBe(false);
	expect(controller.organizations[0].ownerName).toBe('owner');
	expect(controller.organizations[0].openOpportunityCount).toBe(1);
	expect(controller.opportunities[0].expectedValue).toBe(12.345);
	let writes = 0;
	write = async () => { writes += 1; };
	await controller.saveContact(controller.contacts[0]);
	await controller.saveOrganization(controller.organizations[0]);
	await controller.saveOpportunity(controller.opportunities[0]);
	await controller.archiveOpportunity('deal');
	expect(writes).toBe(4);
	pending.resolve({ records: [owner], availableGroups: [] });
	await pending.promise;
	expect(controller.isDirectoryReady).toBe(true);
	expect(controller.organizations[0].ownerName).toBe(owner.name);
	expect(controller.opportunities[0].expectedValue).toBe(12.345);
});

test('directory failures stay separate and retry only the directory', async () => {
	directory = async () => { throw new Error('directory offline'); };
	let reads = 0;
	read = async () => { reads += 1; return snapshot(); };
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	expect(controller.hasData).toBe(true);
	expect(controller.errorMessage).toBe('');
	expect(controller.directoryErrorMessage).toBe(crmText.en.organizationLoadFailed);
	directory = async () => ({ records: [owner], availableGroups: [] });
	await controller.retryDirectory();
	expect(controller.isDirectoryReady).toBe(true);
	expect(controller.directoryErrorMessage).toBe('');
	expect(reads).toBe(1);
});

test('currency metadata remains required before showing amounts', async () => {
	const pending = Promise.withResolvers<currency.CurrencyCatalogue>();
	catalogue = () => pending.promise;
	const controller = new CRMPageController(crmText.en);
	const loading = controller.load(owner.email);
	await Promise.resolve();
	expect(controller.hasData).toBe(false);
	expect(controller.isLoading).toBe(true);
	pending.resolve([{ code: 'BHD', name: 'Bahraini Dinar', minorUnitDigits: 3 }]);
	await loading;
	expect(controller.opportunities[0].expectedValue).toBe(12.345);
});

test('a fallback catalogue cannot silently use two decimal places for an unknown currency', async () => {
	catalogue = async () => currency.interimCurrencyCatalogue;
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	expect(controller.hasData).toBe(false);
	expect(controller.errorMessage).toBe(crmText.en.currencyUnavailable);
});

test('base currency failure is observed even while the directory remains pending', async () => {
	const pending = Promise.withResolvers<Directory>();
	directory = () => pending.promise;
	baseCurrency = async () => { throw new Error('base currency unavailable'); };
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	expect(controller.hasData).toBe(false);
	expect(controller.isLoading).toBe(false);
	expect(controller.errorMessage).toBe('base currency unavailable');
	controller.dispose();
	pending.resolve({ records: [], availableGroups: [] });
});

test('failed mutation refresh keeps visible data and forces the next refresh to read a complete snapshot', async () => {
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	read = async () => { throw new Error('temporarily unavailable'); };
	await controller.saveContact(controller.contacts[0]);
	expect(controller.hasData).toBe(true);
	expect(controller.errorMessage).toBe(crmText.en.refreshAfterSaveFailed);
	read = async (previous) => {
		expect(previous).toBeUndefined();
		return snapshot('Recovered Account');
	};
	await controller.archiveOrganization('organization');
	expect(controller.errorMessage).toBe('');
	expect(controller.organizations[0].name).toBe('Recovered Account');
});

test('a denied refresh hides records instead of treating the denial as an optional lookup error', async () => {
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	read = async () => { throw new CRMApiError('denied', 403, 'denied'); };
	await controller.saveContact(controller.contacts[0]);
	expect(controller.permissionDenied).toBe(true);
	expect(controller.errorMessage).toBe(crmText.en.permissionDenied);
});

test('old load, directory and errors cannot overwrite a newer account or disposed page', async () => {
	const oldData = Promise.withResolvers<CRMDataResponse>();
	const oldDirectory = Promise.withResolvers<Directory>();
	read = () => oldData.promise;
	directory = () => oldDirectory.promise;
	const controller = new CRMPageController(crmText.en);
	const oldLoad = controller.load('old@example.com');
	read = async () => snapshot('New Account');
	directory = async () => ({ records: [owner], availableGroups: [] });
	await controller.load(owner.email);
	oldData.reject(new CRMApiError('denied', 403, 'denied'));
	oldDirectory.resolve({ records: [], availableGroups: [] });
	await oldLoad;
	expect(controller.organizations[0].name).toBe('New Account');
	expect(controller.people).toEqual([owner]);
	expect(controller.permissionDenied).toBe(false);
	const unmounted = Promise.withResolvers<CRMDataResponse>();
	read = () => unmounted.promise;
	const lastLoad = controller.load(owner.email);
	controller.dispose();
	unmounted.resolve(snapshot('Late Account'));
	await lastLoad;
	expect(controller.hasData).toBe(false);
	expect(controller.organizations[0].name).toBe('New Account');
});

test('overlapping mutations serialize fresh snapshots and preserve both changed lists', async () => {
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	const pending = Promise.withResolvers<CRMDataResponse>();
	const started = Promise.withResolvers<void>();
	const writes: string[] = [];
	write = async (name) => { writes.push(name); };
	read = async (previous, changed) => {
		if (changed?.includes('contacts')) { started.resolve(); return pending.promise; }
		expect(previous?.contacts[0].name).toBe('Updated Contact');
		return { ...snapshot('Updated Account'), contacts: previous?.contacts ?? [] };
	};
	const first = controller.saveContact(controller.contacts[0]);
	await started.promise;
	const second = controller.saveOrganization(controller.organizations[0]);
	expect(writes).toEqual(['contact']);
	expect(controller.isSaving).toBe(true);
	const updated = snapshot();
	updated.contacts[0].name = 'Updated Contact';
	pending.resolve(updated);
	await Promise.all([first, second]);
	expect(writes).toEqual(['contact', 'organization']);
	expect(controller.contacts[0].name).toBe('Updated Contact');
	expect(controller.organizations[0].name).toBe('Updated Account');
	expect(controller.isSaving).toBe(false);
});

test('switching accounts cancels queued writes and ignores an in-flight mutation refresh', async () => {
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	const pending = Promise.withResolvers<unknown>();
	const started = Promise.withResolvers<void>();
	let writes = 0;
	write = async () => { writes += 1; started.resolve(); return pending.promise; };
	const first = controller.saveContact(controller.contacts[0]);
	await started.promise;
	const second = controller.saveContact(controller.contacts[0]);
	const outcomes = Promise.allSettled([first, second]);
	read = async () => snapshot('New Account');
	await controller.load('new@example.com');
	pending.resolve({});
	expect((await outcomes).map((result) => result.status)).toEqual(['rejected', 'rejected']);
	expect(writes).toBe(1);
	expect(controller.organizations[0].name).toBe('New Account');
	expect(controller.isSaving).toBe(false);
});

test('a refused save reaches the form that asked and leaves the page without a second alert', async () => {
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	write = async () => { throw new CRMApiError('in use', 409, 'crm_definition_in_use'); };
	await expect(controller.saveVocabulary(controller.vocabulary)).rejects.toThrow(crmText.en.definitionInUse);
	expect(controller.errorMessage).toBe('');
});

test('a refused move on the board has no form, so the page shows it', async () => {
	const controller = new CRMPageController(crmText.en);
	await controller.load(owner.email);
	write = async () => { throw new CRMApiError('conflict', 409, 'conflict'); };
	await expect(controller.moveOpportunity({ opportunityID: 'deal', targetStage: 'waiting', beforeOpportunityID: null })).rejects.toThrow(crmText.en.conflict);
	expect(controller.errorMessage).toBe(crmText.en.conflict);
});
