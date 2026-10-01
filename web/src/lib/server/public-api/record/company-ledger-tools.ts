import { assetBucket } from '../asset-address';
import { titleNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintMatcher } from './hint-resolution';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import type { RecordContext } from './company';
import type {
	CompanyDocumentDownloadResult,
	CompanyDocumentListResult,
	CompanyDocumentPublished,
	CompanyDocumentRegisteredResult,
	CompanyDocumentResult,
	CompanyDocumentUploadResult,
	CompanyMetricListResult,
	CompanyMetricResult,
	CompanyRecordListResult,
	CompanyRecordResult
} from '../catalog/company';

export type CompanyMetricRecordInput = {
	metric?: string;
	year?: number;
	quarter?: number;
	month?: number;
	value?: number;
	currency?: string;
	valueUSD?: number;
	unit?: string;
	note?: string;
};

export type CompanyMetricListInput = { metric?: string; fromYear?: number; toYear?: number };

export type CompanyRecordAddInput = {
	category?: string;
	date?: string;
	title?: string;
	detail?: string;
	attributes?: string;
};

export type CompanyRecordUpdateInput = CompanyRecordAddInput & { recordHint?: string };

export type CompanyRecordDeleteInput = { recordHint?: string };

export type CompanyRecordListInput = { category?: string; query?: string };

export type DataRoomDocumentInput = {
	categoryCode?: string;
	clearance?: number;
	domain?: string;
	date?: string;
	period?: string;
	status?: string;
	supersedesHint?: string;
	sha256?: string;
	tags?: string[];
	storagePath?: string;
};

export type CompanyDocumentRegisterInput = DataRoomDocumentInput & {
	kind?: string;
	documentType?: string;
	title?: string;
	counterpart?: string;
	language?: string;
	filePath?: string;
	summary?: string;
};

export type CompanyDocumentUpdateInput = DataRoomDocumentInput & {
	documentHint?: string;
	title?: string;
	counterpart?: string;
	filePath?: string;
	summary?: string;
};

export type CompanyDocumentListInput = {
	categoryCode?: string;
	type?: string;
	counterpart?: string;
	query?: string;
	domain?: string;
	clearance?: number;
};

export type CompanyDocumentUploadInput = { categoryCode?: string; clearance?: number; sha256?: string; fileName?: string };

export type CompanyDocumentDownloadInput = {
	documentHint?: string;
	storagePath?: string;
	fileName?: string;
};

export type CompanyDocumentSearchInput = { query?: string; limit?: number };

const earliestMetricYear = 1900;
const documentSearchDefaultLimit = 5;
const documentSearchLimit = 50;
const downloadableForTenMinutes = 10 * 60;
const lowestClearance = 0;
const highestClearance = 3;
const sha256Pattern = /^[0-9a-f]{64}$/;

const metricColumns =
	'id, metric, year, quarter, month, value, currency_code, value_usd, unit, note, updated_at';
const recordColumns = 'id, category, record_date, title, detail, attributes, updated_at';
const documentColumns =
	'id, document_number, kind, document_type, title, counterpart, language, file_path, summary, requester_id, issued_at, category_code, clearance, domain, document_date, period, status, supersedes, sha256, tags, storage_path, published_from, published_at, published_by';

const documentNumberPrefixes: Record<string, string> = {
	'quote': 'Q',
	'invoice': 'INV',
	'purchase-order': 'PO',
	'transaction-statement': 'TS',
	'approval-request': 'APR',
	'expense-approval': 'EXP',
	'meeting-minutes': 'MIN',
	'weekly-report': 'WKR',
	'business-trip-report': 'BTR',
	'employment-certificate': 'CERT',
	'career-certificate': 'CRT',
	'leave-request': 'LV',
	'power-of-attorney': 'POA',
	'offer-letter': 'OFR',
	'employment-contract': 'EMP',
	'nda': 'NDA',
	'mou': 'MOU',
	'service-agreement': 'SVC'
};

const documentKinds = ['issued', 'received', 'internal'];

type MetricRow = {
	id: string;
	metric: string;
	year: number;
	quarter: number;
	month: number;
	value: number;
	currency_code: string | null;
	value_usd: number | null;
	unit: string | null;
	note: string | null;
	updated_at: string;
};

type RecordRow = {
	id: string;
	category: string;
	record_date: string | null;
	title: string;
	detail: string | null;
	attributes: Record<string, string> | null;
	updated_at: string;
};

type DocumentRow = {
	id: string;
	document_number: string | null;
	kind: string;
	document_type: string;
	title: string;
	counterpart: string | null;
	language: string | null;
	file_path: string | null;
	summary: string | null;
	requester_id: string | null;
	issued_at: string;
	clearance: number;
	category_code: string | null;
	domain: string | null;
	document_date: string | null;
	period: string | null;
	status: string | null;
	supersedes: string | null;
	sha256: string | null;
	tags: string[] | null;
	storage_path: string | null;
	published_from: string | null;
	published_at: string | null;
	published_by: string | null;
};

function answeredMetric(row: MetricRow): CompanyMetricResult {
	return {
		metricID: row.id,
		metric: row.metric,
		year: row.year,
		quarter: row.quarter,
		month: row.month,
		value: Number(row.value),
		currency: row.currency_code,
		valueUSD: row.value_usd === null ? null : Number(row.value_usd),
		unit: row.unit,
		note: row.note,
		updatedAt: row.updated_at
	};
}

function answeredRecord(row: RecordRow): CompanyRecordResult {
	return {
		recordID: row.id,
		category: row.category,
		date: row.record_date,
		title: row.title,
		detail: row.detail,
		attributes: Object.entries(row.attributes ?? {}).map(([label, value]) => ({ label, value })),
		updatedAt: row.updated_at
	};
}

function answeredDocument(row: DocumentRow): CompanyDocumentResult {
	return {
		documentID: row.id,
		documentNumber: row.document_number,
		kind: row.kind,
		documentType: row.document_type,
		title: row.title,
		counterpart: row.counterpart,
		language: row.language,
		filePath: row.file_path,
		summary: row.summary,
		requesterID: row.requester_id,
		issuedAt: row.issued_at,
		categoryCode: row.category_code,
		clearance: row.clearance,
		domain: row.domain,
		date: row.document_date,
		period: row.period,
		status: row.status,
		supersedes: row.supersedes,
		sha256: row.sha256,
		tags: row.tags ?? [],
		storagePath: row.storage_path,
		published: publishedOf(row)
	};
}

function publishedOf(row: DocumentRow): CompanyDocumentPublished | null {
	if (!row.published_at) return null;
	return { at: row.published_at, by: row.published_by, from: row.published_from };
}

function textOf(value: string | undefined, refusal: string): string {
	const given = value?.trim();
	if (!given) throw new Error(refusal);
	return given;
}

function orNull(value: string | undefined): string | null {
	const given = value?.trim();
	return given ? given : null;
}

function refuseTheWrite(
	error: { message: string; code?: string },
	whenForbidden: string
): RecordRefusedTheWrite {
	const status = statusOfPostgresCode(error.code);
	if (status === 403) return new RecordRefusedTheWrite(whenForbidden, 403);
	if (error.code === '23505') return new RecordRefusedTheWrite(error.message, 409, 'record_duplicate');
	return new RecordRefusedTheWrite(error.message, status);
}

const onlyAnAdministratorWritesMetrics = 'only an administrator can record a company metric';
const onlyAnAdministratorWritesRecords = 'only an administrator can change a company record';
const onlyAColleagueWritesDocuments = 'only somebody who works here can change the document ledger';

function decodedAttributes(attributes: string | undefined): Record<string, string> | undefined {
	const written = attributes?.trim();
	if (!written) return undefined;
	let parsed: unknown;
	try {
		parsed = JSON.parse(written);
	} catch {
		throw new Error(
			'attributes must be a JSON object string of label-to-value pairs, e.g. {"round": "Seed"}'
		);
	}
	if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
		throw new Error('attributes must be a JSON object string of label-to-value pairs');
	}
	const values: Record<string, string> = {};
	for (const [label, value] of Object.entries(parsed as Record<string, unknown>)) {
		if (typeof value === 'string') values[label] = value;
		else if (typeof value === 'number' || typeof value === 'boolean') values[label] = String(value);
		else throw new Error(`attributes.${label} must be a string, a number or a boolean`);
	}
	return values;
}

type MetricMoney = { currencyCode: string | null; valueUSD: number | null; unit: string | null };

function moneyOfMetric(input: CompanyMetricRecordInput, value: number): MetricMoney {
	const currencyCode = input.currency?.trim().toUpperCase() ?? '';
	const unit = orNull(input.unit);
	if (currencyCode === '') {
		if (input.valueUSD !== undefined) {
			throw new Error('a currency is required when a USD equivalent is given');
		}
		return { currencyCode: null, valueUSD: null, unit };
	}
	if (unit !== null) {
		throw new Error('a metric carries a currency for money or a unit for everything else, never both');
	}
	if (currencyCode === 'USD') return { currencyCode, valueUSD: value, unit: null };
	if (input.valueUSD === undefined) {
		throw new Error(`a USD equivalent is required when the currency is ${currencyCode}`);
	}
	return { currencyCode, valueUSD: input.valueUSD, unit: null };
}

function periodOfMetric(input: CompanyMetricRecordInput): { quarter: number; month: number } {
	const quarter = input.quarter ?? 0;
	const month = input.month ?? 0;
	if (quarter !== 0 && month !== 0) {
		throw new Error(
			'a metric is annual, quarterly or monthly: give a quarter or a month, never both'
		);
	}
	if (quarter < 0 || quarter > 4) throw new Error('a quarter is 1 to 4');
	if (month < 0 || month > 12) throw new Error('a month is 1 to 12');
	return { quarter, month };
}

export async function companyMetricRecord(
	context: RecordContext,
	input: CompanyMetricRecordInput
): Promise<CompanyMetricResult> {
	const metric = textOf(input.metric, 'a metric call names the metric it records');
	const year = input.year ?? 0;
	if (year < earliestMetricYear) throw new Error('a metric names the four-digit year it belongs to');
	if (input.value === undefined) throw new Error('a metric records a number');
	const { quarter, month } = periodOfMetric(input);
	const money = moneyOfMetric(input, input.value);

	const { data, error } = await context.caller
		.from('company_metric')
		.upsert(
			{
				company_id: context.companyID,
				metric,
				year,
				quarter,
				month,
				value: input.value,
				currency_code: money.currencyCode,
				value_usd: money.valueUSD,
				unit: money.unit,
				note: orNull(input.note),
				updated_at: context.now.toISOString()
			},
			{ onConflict: 'company_id,metric,year,quarter,month' }
		)
		.select(metricColumns)
		.single<MetricRow>();
	if (error) throw refuseTheWrite(error, onlyAnAdministratorWritesMetrics);
	return answeredMetric(data);
}

export async function companyMetricList(
	context: RecordContext,
	input: CompanyMetricListInput
): Promise<CompanyMetricListResult> {
	let query = context.caller
		.from('company_metric')
		.select(metricColumns)
		.eq('company_id', context.companyID);
	const metric = orNull(input.metric);
	if (metric) query = query.eq('metric', metric);
	if (input.fromYear !== undefined) query = query.gte('year', input.fromYear);
	if (input.toYear !== undefined) query = query.lte('year', input.toYear);

	const { data, error } = await query
		.order('metric')
		.order('year')
		.order('quarter')
		.order('month')
		.returns<MetricRow[]>();
	if (error) throw new Error(error.message);
	const metrics = (data ?? []).map(answeredMetric);
	return { count: metrics.length, metrics };
}

const recordMatcher: HintMatcher<RecordRow> = {
	identifiersOf: (row) => [row.id],
	titleOf: (row) => row.title,
	nearnessTo: (row, hint) => titleNearness(normalized(hint), normalized(row.title))
};

async function recordRows(context: RecordContext): Promise<RecordRow[]> {
	const { data, error } = await context.caller
		.from('company_record')
		.select(recordColumns)
		.eq('company_id', context.companyID)
		.order('record_date', { ascending: false, nullsFirst: false })
		.order('updated_at', { ascending: false })
		.returns<RecordRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

export async function companyRecordOfHint(
	context: RecordContext,
	hint: string
): Promise<RecordRow> {
	const asked = hint.trim();
	if (!asked) throw new Error('this call names the company record it is about');
	const resolution = resolveHint(asked, await recordRows(context), recordMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'record',
		asked,
		resolution.outcome,
		resolution.candidates.map((row) => ({
			id: row.id,
			label: row.record_date ? `${row.record_date} ${row.title}` : row.title
		}))
	);
}

export async function companyRecordAdd(
	context: RecordContext,
	input: CompanyRecordAddInput
): Promise<CompanyRecordResult> {
	const category = textOf(input.category, 'a company record names the category it belongs to');
	const title = textOf(input.title, 'a company record needs a title');

	const { data, error } = await context.caller
		.from('company_record')
		.insert({
			company_id: context.companyID,
			category,
			record_date: orNull(input.date),
			title,
			detail: orNull(input.detail),
			attributes: decodedAttributes(input.attributes) ?? {},
			updated_at: context.now.toISOString()
		})
		.select(recordColumns)
		.single<RecordRow>();
	if (error) throw refuseTheWrite(error, onlyAnAdministratorWritesRecords);
	return answeredRecord(data);
}

export async function companyRecordUpdate(
	context: RecordContext,
	input: CompanyRecordUpdateInput
): Promise<CompanyRecordResult> {
	const held = await companyRecordOfHint(context, input.recordHint ?? '');
	const attributes = decodedAttributes(input.attributes);
	const change: Record<string, unknown> = { updated_at: context.now.toISOString() };
	if (orNull(input.category)) change.category = input.category?.trim();
	if (orNull(input.date)) change.record_date = input.date?.trim();
	if (orNull(input.title)) change.title = input.title?.trim();
	if (orNull(input.detail)) change.detail = input.detail?.trim();
	if (attributes) change.attributes = attributes;

	const { data, error } = await context.caller
		.from('company_record')
		.update(change)
		.eq('id', held.id)
		.select(recordColumns)
		.returns<RecordRow[]>();
	if (error) throw refuseTheWrite(error, onlyAnAdministratorWritesRecords);
	const written = (data ?? [])[0];
	if (!written) throw new RecordRefusedTheWrite(onlyAnAdministratorWritesRecords, 403);
	return answeredRecord(written);
}

export async function companyRecordDelete(
	context: RecordContext,
	input: CompanyRecordDeleteInput
): Promise<CompanyRecordResult> {
	const held = await companyRecordOfHint(context, input.recordHint ?? '');

	const { data, error } = await context.caller
		.from('company_record')
		.delete()
		.eq('id', held.id)
		.select(recordColumns)
		.returns<RecordRow[]>();
	if (error) throw refuseTheWrite(error, onlyAnAdministratorWritesRecords);
	if ((data ?? []).length === 0) {
		throw new RecordRefusedTheWrite(onlyAnAdministratorWritesRecords, 403);
	}
	return answeredRecord(held);
}

export async function companyRecordList(
	context: RecordContext,
	input: CompanyRecordListInput
): Promise<CompanyRecordListResult> {
	const category = orNull(input.category);
	const keyword = orNull(input.query)?.toLowerCase();
	const kept = (await recordRows(context))
		.filter((row) => !category || row.category === category)
		.filter((row) => !keyword || recordHolds(row, keyword));
	return { count: kept.length, records: kept.map(answeredRecord) };
}

function recordHolds(row: RecordRow, keyword: string): boolean {
	const written = [row.title, row.detail ?? '', ...Object.values(row.attributes ?? {})];
	return written.some((value) => value.toLowerCase().includes(keyword));
}

const documentMatcher: HintMatcher<DocumentRow> = {
	identifiersOf: (row) => [row.id, row.document_number ?? ''],
	titleOf: (row) => row.title,
	nearnessTo: (row, hint) => titleNearness(normalized(hint), normalized(row.title))
};

async function documentRows(context: RecordContext): Promise<DocumentRow[]> {
	const { data, error } = await context.caller
		.from('company_document')
		.select(documentColumns)
		.eq('company_id', context.companyID)
		.order('issued_at', { ascending: false })
		.returns<DocumentRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

export async function companyDocumentOfHint(
	context: RecordContext,
	hint: string
): Promise<DocumentRow> {
	const asked = hint.trim();
	if (!asked) throw new Error('this call names the document it is about');
	return documentAmong(await documentRows(context), asked);
}

function documentAmong(rows: DocumentRow[], asked: string): DocumentRow {
	const resolution = resolveHint(asked, rows, documentMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused('document', asked, resolution.outcome, resolution.candidates.map(documentCandidateOf));
}

function documentCandidateOf(row: DocumentRow): { id: string; label: string } {
	return {
		id: row.id,
		label: row.document_number ? `${row.document_number} ${row.title}` : row.title
	};
}

function documentNumberPrefix(documentType: string): string {
	const known = documentNumberPrefixes[documentType];
	if (known) return known;
	const initials = documentType
		.split('-')
		.map((segment) => segment.match(/[a-z]/i)?.[0] ?? '')
		.join('')
		.toUpperCase();
	return initials || 'DOC';
}

function kindOfDocument(kind: string | undefined): string {
	const asked = kind?.trim().toLowerCase() ?? '';
	return documentKinds.includes(asked) ? asked : 'issued';
}

export async function companyDocumentRegister(
	context: RecordContext,
	input: CompanyDocumentRegisterInput
): Promise<CompanyDocumentRegisteredResult> {
	const documentType = textOf(input.documentType, 'a document names the type it is');
	const title = textOf(input.title, 'a document needs a title');
	const kind = kindOfDocument(input.kind);
	const prefix = `${documentNumberPrefix(documentType)}-${context.now.getUTCFullYear()}-`;

	const written = {
		company_id: context.companyID,
		kind,
		document_type: documentType,
		title,
		counterpart: orNull(input.counterpart),
		language: orNull(input.language),
		file_path: orNull(input.filePath),
		summary: orNull(input.summary),
		requester_id: context.requesterID,
		issued_at: context.now.toISOString(),
		...(await dataRoomChangeOf(context, input, orNull(input.domain))),
		category_code: input.categoryCode ?? (input.domain || input.clearance !== undefined ? null : 'X')
	};

	const documentNumber = kind === 'issued' ? await reservedDocumentNumber(context, prefix) : null;
	const { data, error } = await context.caller
		.from('company_document')
		.insert({ ...written, document_number: documentNumber })
		.select(documentColumns)
		.single<DocumentRow>();
	if (error) throw refuseTheWrite(error, onlyAColleagueWritesDocuments);
	return {
		...answeredDocument(data),
		storageDirectory: `/workspace/circles/member/documents/${documentType}`
	};
}

async function reservedDocumentNumber(context: RecordContext, prefix: string): Promise<string> {
	const { data, error } = await context.caller.rpc('reserve_document_number', {
		target_company: context.companyID,
		requested_prefix: prefix
	});
	if (error) throw refuseTheWrite(error, onlyAColleagueWritesDocuments);
	return String(data);
}

export async function companyDocumentUpdate(
	context: RecordContext,
	input: CompanyDocumentUpdateInput
): Promise<CompanyDocumentResult> {
	const held = await companyDocumentOfHint(context, input.documentHint ?? '');
	const domain = orNull(input.domain) ?? held.domain;
	const change: Record<string, unknown> = await dataRoomChangeOf(context, input, domain, held.category_code);
	if (orNull(input.title)) change.title = input.title?.trim();
	if (orNull(input.counterpart)) change.counterpart = input.counterpart?.trim();
	if (orNull(input.filePath)) change.file_path = input.filePath?.trim();
	if (orNull(input.summary)) change.summary = input.summary?.trim();
	if (Object.keys(change).length === 0) {
		throw new Error('a document update names at least one thing to change');
	}

	const { data, error } = await context.caller
		.from('company_document')
		.update(change)
		.eq('id', held.id)
		.select(documentColumns)
		.returns<DocumentRow[]>();
	if (error) throw refuseTheWrite(error, onlyAColleagueWritesDocuments);
	const written = (data ?? [])[0];
	if (!written) throw new RecordRefusedTheWrite(onlyAColleagueWritesDocuments, 403);
	return answeredDocument(written);
}

export async function companyDocumentList(
	context: RecordContext,
	input: CompanyDocumentListInput
): Promise<CompanyDocumentListResult> {
	const documentType = orNull(input.type);
	const counterpart = orNull(input.counterpart)?.toLowerCase();
	const keyword = orNull(input.query)?.toLowerCase();
	const domain = orNull(input.domain);
	const clearance = input.clearance === undefined ? undefined : clearanceOf(input.clearance);
	const kept = (await documentRows(context))
		.filter((row) => !input.categoryCode || row.category_code === input.categoryCode
			|| (input.categoryCode.length === 1 && row.category_code?.startsWith(input.categoryCode)))
		.filter((row) => !documentType || row.document_type === documentType)
		.filter((row) => !counterpart || (row.counterpart ?? '').toLowerCase().includes(counterpart))
		.filter((row) => !domain || row.domain === domain)
		.filter((row) => clearance === undefined || row.clearance === clearance)
		.filter((row) => !keyword || documentHolds(row, keyword));
	return { count: kept.length, documents: kept.map(answeredDocument) };
}

function documentHolds(row: DocumentRow, keyword: string): boolean {
	const written = [row.title, row.summary ?? '', row.counterpart ?? ''];
	return written.some((value) => value.toLowerCase().includes(keyword));
}

export async function companyDocumentSearch(
	context: RecordContext,
	input: CompanyDocumentSearchInput
): Promise<CompanyDocumentListResult> {
	const asked = textOf(input.query, 'a document search asks something');
	const limit = Math.min(Math.max(input.limit ?? documentSearchDefaultLimit, 1), documentSearchLimit);

	const { data, error } = await context.caller.rpc('company_documents_matching', {
		target_query: asked,
		target_limit: limit
	});
	if (error) throw new Error(error.message);
	if (!Array.isArray(data)) throw new Error('a document search answers a list of documents');
	const documents = (data as DocumentRow[]).map(answeredDocument);
	return { count: documents.length, documents };
}

function clearanceOf(clearance: number): number {
	if (!Number.isInteger(clearance) || clearance < lowestClearance || clearance > highestClearance) {
		throw new Error(`a data room clearance is a whole number from ${lowestClearance} to ${highestClearance}`);
	}
	return clearance;
}

function sha256Of(sha256: string | undefined): string {
	const given = sha256?.trim().toLowerCase() ?? '';
	if (!sha256Pattern.test(given)) throw new Error('a sha256 is 64 lowercase hex characters');
	return given;
}

function derivedFileNameOf(fileName: string | undefined): string | null {
	const given = orNull(fileName);
	if (given === null) return null;
	if (given === '.' || given === '..' || given.includes('/')) {
		throw new Error('a derived file name is one name beside the original, never a path');
	}
	return given;
}

function tagsOf(tags: string[]): string[] {
	return tags.map((tag) => tag.trim()).filter(Boolean);
}

async function dataRoomChangeOf(
	context: RecordContext,
	input: DataRoomDocumentInput,
	domain: string | null,
	categoryCode: string | null = null
): Promise<Record<string, unknown>> {
	const change: Record<string, unknown> = {};
	if (input.categoryCode !== undefined) change.category_code = input.categoryCode;
	if (input.clearance !== undefined) change.clearance = clearanceOf(input.clearance);
	if (orNull(input.domain)) change.domain = input.domain?.trim();
	if (orNull(input.date)) change.document_date = input.date?.trim();
	if (orNull(input.period)) change.period = input.period?.trim();
	if (orNull(input.status)) change.status = input.status?.trim();
	if (input.sha256 !== undefined) change.sha256 = sha256Of(input.sha256);
	if (input.tags !== undefined) change.tags = tagsOf(input.tags);
	if (orNull(input.storagePath)) change.storage_path = input.storagePath?.trim();
	if (orNull(input.supersedesHint)) {
		change.supersedes = (await supersededDocumentOfHint(context, input.supersedesHint ?? '', domain, input.categoryCode ?? categoryCode)).id;
	}
	return change;
}

async function supersededDocumentOfHint(
	context: RecordContext,
	hint: string,
	domain: string | null,
	categoryCode: string | null
): Promise<DocumentRow> {
	const candidates = (await documentRows(context)).filter((row) =>
		categoryCode ? row.category_code === categoryCode : row.category_code === null && row.domain === domain);
	return documentAmong(candidates, hint.trim());
}

function dataRoomObjectPath(companyID: string, category: string | number, sha256: string, fileName: string | null): string {
	const original = `${companyID}/dataroom/${category}/${sha256}`;
	return fileName === null ? original : `${original}/${fileName}`;
}

export async function companyDocumentUpload(
	context: RecordContext,
	input: CompanyDocumentUploadInput
): Promise<CompanyDocumentUploadResult> {
	if (input.categoryCode !== undefined && input.clearance !== undefined) {
		throw new RecordRefusedTheWrite('choose categoryCode for new files or clearance for a legacy file, not both', 400);
	}
	const category = input.categoryCode ?? (input.clearance === undefined ? 'X' : clearanceOf(input.clearance));
	const storagePath = dataRoomObjectPath(
		context.companyID,
		category,
		sha256Of(input.sha256),
		derivedFileNameOf(input.fileName)
	);

	const { data, error } = await context.caller.storage
		.from(assetBucket)
		.createSignedUploadUrl(storagePath, { upsert: true });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfStorageError(error));
	return { storagePath, uploadURL: data.signedUrl };
}

export async function companyDocumentDownload(
	context: RecordContext,
	input: CompanyDocumentDownloadInput
): Promise<CompanyDocumentDownloadResult> {
	const original = await storedOriginalOf(context, input);
	const fileName = derivedFileNameOf(input.fileName);
	const storagePath = fileName === null ? original : `${original}/${fileName}`;

	const { data, error } = await context.caller.storage
		.from(assetBucket)
		.createSignedUrl(storagePath, downloadableForTenMinutes);
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfStorageError(error));
	return { storagePath, downloadURL: data.signedUrl };
}

async function storedOriginalOf(
	context: RecordContext,
	input: CompanyDocumentDownloadInput
): Promise<string> {
	const storagePath = orNull(input.storagePath);
	if (storagePath) return storagePath;
	if (!orNull(input.documentHint)) throw new Error('a download names the document or its storagePath');
	const held = await companyDocumentOfHint(context, input.documentHint ?? '');
	if (!held.storage_path) {
		throw new RecordRefusedTheWrite('this document keeps no file in the data room', 404, 'company_document_no_file');
	}
	return held.storage_path;
}

function statusOfStorageError(error: { status?: number }): number {
	if (error.status === 400 || error.status === 403 || error.status === 404) return error.status;
	return 502;
}
