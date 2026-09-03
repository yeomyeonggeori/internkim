import { titleNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintMatcher } from './hint-resolution';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import type { RecordContext } from './company';
import type {
	CompanyDocumentListResult,
	CompanyDocumentRegisteredResult,
	CompanyDocumentResult,
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

export type CompanyDocumentRegisterInput = {
	kind?: string;
	documentType?: string;
	title?: string;
	counterpart?: string;
	language?: string;
	filePath?: string;
	summary?: string;
};

export type CompanyDocumentUpdateInput = {
	documentHint?: string;
	title?: string;
	counterpart?: string;
	filePath?: string;
	summary?: string;
};

export type CompanyDocumentListInput = { type?: string; counterpart?: string; query?: string };

export type CompanyDocumentSearchInput = { query?: string; limit?: number };

const earliestMetricYear = 1900;
const documentSearchDefaultLimit = 5;
const documentSearchLimit = 50;
const documentNumberAttempts = 5;

const metricColumns =
	'id, metric, year, quarter, month, value, currency_code, value_usd, unit, note, updated_at';
const recordColumns = 'id, category, record_date, title, detail, attributes, updated_at';
const documentColumns =
	'id, document_number, kind, document_type, title, counterpart, language, file_path, summary, requester_id, issued_at';

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
		issuedAt: row.issued_at
	};
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
	const resolution = resolveHint(asked, await documentRows(context), documentMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'document',
		asked,
		resolution.outcome,
		resolution.candidates.map((row) => ({
			id: row.id,
			label: row.document_number ? `${row.document_number} ${row.title}` : row.title
		}))
	);
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

function nextDocumentNumber(held: DocumentRow[], prefix: string): string {
	const highest = held.reduce((standing, row) => {
		const number = row.document_number ?? '';
		if (!number.startsWith(prefix)) return standing;
		const sequence = Number.parseInt(number.slice(prefix.length), 10);
		return Number.isNaN(sequence) ? standing : Math.max(standing, sequence);
	}, 0);
	return `${prefix}${String(highest + 1).padStart(3, '0')}`;
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
		issued_at: context.now.toISOString()
	};

	for (let attempt = 0; attempt < documentNumberAttempts; attempt += 1) {
		const documentNumber = kind === 'issued' ? nextDocumentNumber(await documentRows(context), prefix) : null;
		const { data, error } = await context.caller
			.from('company_document')
			.insert({ ...written, document_number: documentNumber })
			.select(documentColumns)
			.single<DocumentRow>();
		if (!error) {
			return {
				...answeredDocument(data),
				storageDirectory: `/workspace/circles/member/documents/${documentType}`
			};
		}
		if (error.code !== '23505' || documentNumber === null) {
			throw refuseTheWrite(error, onlyAColleagueWritesDocuments);
		}
	}
	throw new RecordRefusedTheWrite(
		`the ledger kept handing out ${prefix} numbers somebody else took first`,
		409,
		'record_duplicate'
	);
}

export async function companyDocumentUpdate(
	context: RecordContext,
	input: CompanyDocumentUpdateInput
): Promise<CompanyDocumentResult> {
	const held = await companyDocumentOfHint(context, input.documentHint ?? '');
	const change: Record<string, unknown> = {};
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
	const kept = (await documentRows(context))
		.filter((row) => !documentType || row.document_type === documentType)
		.filter((row) => !counterpart || (row.counterpart ?? '').toLowerCase().includes(counterpart))
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
