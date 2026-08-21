import type {
	CRMOrganizationPayload,
	CRMOrganizationResponse,
	CRMActivityPayload,
	CRMActivityResponse,
	CRMContactPayload,
	CRMContactResponse,
	CRMDataResponse,
	CRMLostReasonResponse,
	CRMOpportunityPayload,
	CRMOpportunityResponse,
	CRMPositionPayload,
	CRMPipelineResponse,
	CRMPipelineStageResponse,
	CRMTransitionPayload
} from './crm-api-types';
import {
	crmOrganizationTypes,
	type CRMOrganizationStatus,
	type CRMOrganizationType,
	deviceCRMActivityKinds,
	type CRMActivityKind,
	type CRMCurrency,
	type CRMImportance,
	type CRMProgressKind
} from './crm-types';

type CRMStageDefinition = {
	id: string;
	outcome: 'open' | 'won' | 'lost' | 'on_hold';
	position: number;
};

type CRMDefinitions = {
	businesses: string[];
	stages: CRMStageDefinition[];
};

export class CRMApiError extends Error {
	constructor(
		message: string,
		readonly status: number,
		readonly code: string
	) {
		super(message);
		this.name = 'CRMApiError';
	}
}

export async function loadCRMData(): Promise<CRMDataResponse> {
	const [organizations, contacts, opportunities, activities, pipelines, lostReasons, definitions] = await Promise.all([
		listDocument('/crm/api/accounts', 'accounts', parseOrganization),
		listDocument('/crm/api/contacts', 'contacts', parseContact),
		listDocument('/crm/api/opportunities', 'opportunities', parseOpportunity),
		listDocument('/crm/api/activities', 'activities', parseActivity),
		listDocument('/crm/api/pipelines', 'pipelines', parsePipeline),
		listDocument('/crm/api/lost-reasons', 'lostReasons', parseLostReason),
		fetchCRMDefinitions()
	]);
	const stages: CRMPipelineStageResponse[] = [...definitions.stages]
		.sort((left, right) => left.position - right.position)
		.map((stage) => ({ stage: stage.id, label: stage.id, position: stage.position, outcome: stage.outcome }));
	return {
		organizations,
		contacts,
		opportunities,
		activities,
		pipelines,
		stages,
		lostReasons,
		vocabulary: {
			organization_types: crmOrganizationTypes.map((id) => ({ id, name: id })),
			pipelines: pipelines.map((pipeline) => ({
				id: pipeline.pipeline,
				name: pipeline.label,
				direction: pipeline.direction
			})),
			stages: stages.map((stage) => ({ id: stage.stage, name: stage.stage, outcome: stage.outcome })),
			lost_reasons: lostReasons.map((reason) => ({ id: reason.reason, name: reason.label }))
		},
		taskVocabulary: {
			businesses: definitions.businesses.map((name) => ({ name })),
			types: deviceCRMActivityKinds.map((name) => ({ name }))
		}
	};
}

export async function createCRMOrganization(payload: CRMOrganizationPayload): Promise<CRMOrganizationResponse> {
	return recordDocument('/crm/api/accounts', 'POST', payload, 'account', parseOrganization);
}

export async function updateCRMOrganization(id: string, payload: CRMOrganizationPayload): Promise<CRMOrganizationResponse> {
	return recordDocument(`/crm/api/accounts/${encodeURIComponent(id)}`, 'PUT', payload, 'account', parseOrganization);
}

export async function archiveCRMOrganization(id: string): Promise<void> {
	await mutationDocument(`/crm/api/accounts/${encodeURIComponent(id)}/archive`, {});
}

export async function createCRMContact(payload: CRMContactPayload): Promise<CRMContactResponse> {
	return recordDocument('/crm/api/contacts', 'POST', payload, 'contact', parseContact);
}

export async function updateCRMContact(id: string, payload: CRMContactPayload): Promise<CRMContactResponse> {
	return recordDocument(`/crm/api/contacts/${encodeURIComponent(id)}`, 'PUT', payload, 'contact', parseContact);
}

export async function createCRMOpportunity(payload: CRMOpportunityPayload): Promise<CRMOpportunityResponse> {
	return recordDocument('/crm/api/opportunities', 'POST', payload, 'opportunity', parseOpportunity);
}

export async function updateCRMOpportunity(id: string, payload: CRMOpportunityPayload): Promise<CRMOpportunityResponse> {
	return recordDocument(`/crm/api/opportunities/${encodeURIComponent(id)}`, 'PUT', payload, 'opportunity', parseOpportunity);
}

export async function archiveCRMOpportunity(id: string): Promise<void> {
	await mutationDocument(`/crm/api/opportunities/${encodeURIComponent(id)}/archive`, {});
}

export async function transitionCRMOpportunity(id: string, payload: CRMTransitionPayload): Promise<CRMOpportunityResponse> {
	return recordDocument(
		`/crm/api/opportunities/${encodeURIComponent(id)}/transition`,
		'POST',
		payload,
		'opportunity',
		parseOpportunity
	);
}

export async function positionCRMOpportunity(id: string, payload: CRMPositionPayload): Promise<CRMOpportunityResponse> {
	return recordDocument(
		`/crm/api/opportunities/${encodeURIComponent(id)}/position`,
		'POST',
		payload,
		'opportunity',
		parseOpportunity
	);
}

export async function createCRMActivity(payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	return recordDocument('/crm/api/activities', 'POST', payload, 'activity', parseActivity);
}

export async function updateCRMActivity(id: string, payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	return recordDocument(`/crm/api/activities/${encodeURIComponent(id)}`, 'PUT', payload, 'activity', parseActivity);
}

async function listDocument<T>(path: string, key: string, parse: (value: unknown) => T): Promise<T[]> {
	const document = await requestDocument(path);
	const values = document[key];
	if (!Array.isArray(values)) throw invalidResponse(path, `${key} must be an array`);
	return values.map(parse);
}

async function fetchCRMDefinitions(): Promise<CRMDefinitions> {
	const document = await requestDocument('/crm/api/definitions');
	return parseDefinitions(document.definitions);
}

function parseDefinitions(value: unknown): CRMDefinitions {
	const record = requiredRecord(value, 'definitions');
	return { businesses: stringArray(record, 'businesses'), stages: stageDefinitionArray(record, 'stages') };
}

function stageDefinitionArray(record: Record<string, unknown>, key: string): CRMStageDefinition[] {
	const values = record[key];
	if (!Array.isArray(values)) throw invalidResponse('CRM API', `${key} must be an array`);
	return values.map(parseStageDefinition);
}

function parseStageDefinition(value: unknown): CRMStageDefinition {
	const record = requiredRecord(value, 'stage definition');
	return {
		id: requiredString(record, 'id'),
		outcome: enumString(record, 'outcome', ['open', 'won', 'lost', 'on_hold']),
		position: requiredNumber(record, 'position')
	};
}

async function recordDocument<T>(
	path: string,
	method: 'POST' | 'PUT',
	payload: object,
	key: string,
	parse: (value: unknown) => T
): Promise<T> {
	const document = await requestDocument(path, {
		method,
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(deviceWireBody(payload))
	});
	return parse(document[key]);
}

function deviceWireBody(payload: object): object {
	if (!('organizationID' in payload)) return payload;
	const { organizationID, ...rest } = payload as { organizationID?: string } & Record<string, unknown>;
	return { ...rest, accountID: organizationID };
}

async function mutationDocument(path: string, payload: object): Promise<Record<string, unknown>> {
	return requestDocument(path, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(payload)
	});
}

async function requestDocument(path: string, init?: RequestInit): Promise<Record<string, unknown>> {
	const response = await fetch(path, { ...init, credentials: 'include' });
	const value: unknown = await response.json().catch(() => null);
	if (!response.ok) {
		const errorDocument = isRecord(value) && isRecord(value.error) ? value.error : {};
		const message = typeof errorDocument.message === 'string' ? errorDocument.message : `CRM API returned ${response.status}`;
		const code = typeof errorDocument.code === 'string' ? errorDocument.code : 'request_failed';
		throw new CRMApiError(message, response.status, code);
	}
	if (!isRecord(value)) throw invalidResponse(path, 'response must be an object');
	return value;
}

function parseOrganization(value: unknown): CRMOrganizationResponse {
	const record = requiredRecord(value, 'account');
	return {
		id: requiredString(record, 'id'),
		name: requiredString(record, 'name'),
		status: enumString(record, 'status', ['prospect', 'active', 'paused']),
		types: enumStringArray(record, 'types', crmOrganizationTypes),
		tags: stringArray(record, 'tags'),
		importance: enumString(record, 'importance', ['high', 'medium', 'low']),
		ownerPersonID: requiredString(record, 'ownerPersonID'),
		ownerCircleID: optionalString(record, 'ownerCircleID'),
		address: optionalString(record, 'address'),
		description: optionalString(record, 'description'),
		audit: parseAudit(record.audit)
	};
}

function parseContact(value: unknown): CRMContactResponse {
	const record = requiredRecord(value, 'contact');
	return {
		id: requiredString(record, 'id'),
		organizationID: requiredString(record, 'accountID'),
		name: requiredString(record, 'name'),
		email: optionalString(record, 'email'),
		phone: optionalString(record, 'phone'),
		title: optionalString(record, 'title'),
		department: optionalString(record, 'department'),
		ownerPersonID: requiredString(record, 'ownerPersonID'),
		ownerCircleID: optionalString(record, 'ownerCircleID'),
		description: optionalString(record, 'description'),
		audit: parseAudit(record.audit)
	};
}

function parseOpportunity(value: unknown): CRMOpportunityResponse {
	const record = requiredRecord(value, 'opportunity');
	return {
		id: requiredString(record, 'id'),
		organizationID: optionalString(record, 'accountID'),
		business: optionalString(record, 'business'),
		name: requiredString(record, 'name'),
		pipeline: enumString(record, 'pipeline', ['sales', 'fundraising', 'investment', 'sponsorship', 'partnership', 'procurement']),
		stage: requiredString(record, 'stage'),
		stagePosition: requiredNumber(record, 'stagePosition'),
		stageChangedAt: requiredString(record, 'stageChangedAt'),
		ownerPersonID: requiredString(record, 'ownerPersonID'),
		ownerCircleID: optionalString(record, 'ownerCircleID'),
		amountMinor: optionalNumber(record, 'amountMinor'),
		currencyCode: currencyCodeString(record, 'currencyCode'),
		baseAmountMinor: optionalNumber(record, 'baseAmountMinor'),
		baseCurrencyCode: optionalCurrencyCodeString(record, 'baseCurrencyCode'),
		importance: enumString(record, 'importance', ['high', 'medium', 'low']),
		dueAt: optionalString(record, 'dueAt'),
		dueTimeZone: optionalString(record, 'dueTimeZone'),
		lostReason: optionalString(record, 'lostReason'),
		description: optionalString(record, 'description'),
		contacts: optionalRecordArray(record, 'contacts', parseOpportunityContact),
		audit: parseAudit(record.audit)
	};
}

function parseOpportunityContact(value: unknown): { contactID: string } {
	const record = requiredRecord(value, 'opportunity contact');
	return { contactID: requiredString(record, 'contactID') };
}

function parseActivity(value: unknown): CRMActivityResponse {
	const record = requiredRecord(value, 'activity');
	return {
		id: requiredString(record, 'id'),
		organizationID: optionalString(record, 'accountID'),
		contactID: optionalString(record, 'contactID'),
		opportunityID: optionalString(record, 'opportunityID'),
		business: optionalString(record, 'business'),
		kind: enumString(record, 'kind', ['note', 'email', 'meeting', 'call', 'task', 'file', 'event', 'stage_change']),
		title: requiredString(record, 'title'),
		occurredAt: requiredString(record, 'occurredAt'),
		content: optionalString(record, 'content'),
		taskStatus: optionalString(record, 'taskStatus'),
		taskOwnerID: optionalString(record, 'taskOwnerID'),
		isEvent: optionalBoolean(record, 'isEvent'),
		isWholeDay: optionalBoolean(record, 'isWholeDay'),
		startsAt: optionalString(record, 'startsAt'),
		endsAt: optionalString(record, 'endsAt'),
		notifyMinutesBefore: optionalNumber(record, 'notifyMinutesBefore'),
		location: optionalString(record, 'location'),
		audit: parseAudit(record.audit)
	};
}

function parsePipeline(value: unknown): CRMPipelineResponse {
	const record = requiredRecord(value, 'pipeline');
	return {
		pipeline: enumString(record, 'pipeline', ['sales', 'fundraising', 'investment', 'sponsorship', 'partnership', 'procurement']),
		label: requiredString(record, 'label'),
		direction: requiredString(record, 'direction'),
		isActive: requiredBoolean(record, 'isActive')
	};
}

function parseLostReason(value: unknown): CRMLostReasonResponse {
	const record = requiredRecord(value, 'lost reason');
	return {
		reason: requiredString(record, 'reason'),
		label: requiredString(record, 'label'),
		isActive: requiredBoolean(record, 'isActive')
	};
}

function parseAudit(value: unknown) {
	const record = requiredRecord(value, 'audit');
	return {
		createdAt: requiredString(record, 'createdAt'),
		createdByPersonID: requiredString(record, 'createdByPersonID'),
		updatedAt: requiredString(record, 'updatedAt'),
		updatedByPersonID: requiredString(record, 'updatedByPersonID'),
		archivedAt: optionalString(record, 'archivedAt'),
		archivedByPersonID: optionalString(record, 'archivedByPersonID')
	};
}

function requiredRecord(value: unknown, label: string): Record<string, unknown> {
	if (!isRecord(value)) throw invalidResponse('CRM API', `${label} must be an object`);
	return value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requiredString(record: Record<string, unknown>, key: string): string {
	const value = record[key];
	if (typeof value !== 'string') throw invalidResponse('CRM API', `${key} must be a string`);
	return value;
}

function optionalString(record: Record<string, unknown>, key: string): string | undefined {
	const value = record[key];
	if (value === undefined || value === null) return undefined;
	if (typeof value !== 'string') throw invalidResponse('CRM API', `${key} must be a string`);
	return value || undefined;
}

function requiredNumber(record: Record<string, unknown>, key: string): number {
	const value = record[key];
	if (typeof value !== 'number' || !Number.isFinite(value)) throw invalidResponse('CRM API', `${key} must be a number`);
	return value;
}

function optionalNumber(record: Record<string, unknown>, key: string): number | undefined {
	const value = record[key];
	if (value === undefined || value === null) return undefined;
	if (typeof value !== 'number' || !Number.isFinite(value)) throw invalidResponse('CRM API', `${key} must be a number`);
	return value;
}

function requiredBoolean(record: Record<string, unknown>, key: string): boolean {
	const value = record[key];
	if (typeof value !== 'boolean') throw invalidResponse('CRM API', `${key} must be a boolean`);
	return value;
}

function optionalBoolean(record: Record<string, unknown>, key: string): boolean | undefined {
	const value = record[key];
	if (value === undefined || value === null) return undefined;
	if (typeof value !== 'boolean') throw invalidResponse('CRM API', `${key} must be a boolean`);
	return value;
}

function stringArray(record: Record<string, unknown>, key: string): string[] {
	const values = record[key];
	if (!Array.isArray(values) || values.some((value) => typeof value !== 'string')) {
		throw invalidResponse('CRM API', `${key} must be a string array`);
	}
	return values;
}

function enumString<const T extends string>(record: Record<string, unknown>, key: string, values: readonly T[]): T {
	const value = requiredString(record, key);
	if (!values.includes(value as T)) throw invalidResponse('CRM API', `${key} has an unsupported value`);
	return value as T;
}

const isoCurrencyCodePattern = /^[A-Z]{3}$/;

function currencyCodeString(record: Record<string, unknown>, key: string): string {
	const value = optionalString(record, key) ?? '';
	if (value !== '' && !isoCurrencyCodePattern.test(value)) throw invalidResponse('CRM API', `${key} is not an ISO 4217 code`);
	return value;
}

function optionalCurrencyCodeString(record: Record<string, unknown>, key: string): string | undefined {
	const value = currencyCodeString(record, key);
	return value === '' ? undefined : value;
}

function optionalEnumString<const T extends string>(record: Record<string, unknown>, key: string, values: readonly T[]): T | undefined {
	const value = optionalString(record, key);
	if (value === undefined) return undefined;
	if (!values.includes(value as T)) throw invalidResponse('CRM API', `${key} has an unsupported value`);
	return value as T;
}

function enumStringArray<const T extends string>(record: Record<string, unknown>, key: string, values: readonly T[]): T[] {
	const items = stringArray(record, key);
	if (items.some((item) => !values.includes(item as T))) throw invalidResponse('CRM API', `${key} has an unsupported value`);
	return items as T[];
}

function optionalRecordArray<T>(record: Record<string, unknown>, key: string, parse: (value: unknown) => T): T[] | undefined {
	const values = record[key];
	if (values === undefined || values === null) return undefined;
	if (!Array.isArray(values)) throw invalidResponse('CRM API', `${key} must be an array`);
	return values.map(parse);
}

function invalidResponse(source: string, reason: string): CRMApiError {
	return new CRMApiError(`${source} response is invalid: ${reason}`, 502, 'invalid_response');
}

export type {
	CRMOrganizationPayload,
	CRMOrganizationResponse,
	CRMActivityPayload,
	CRMActivityResponse,
	CRMContactPayload,
	CRMContactResponse,
	CRMOpportunityPayload,
	CRMOpportunityResponse,
	CRMPositionPayload,
	CRMTransitionPayload,
	CRMOrganizationStatus,
	CRMOrganizationType,
	CRMActivityKind,
	CRMCurrency,
	CRMImportance,
	CRMProgressKind
};
