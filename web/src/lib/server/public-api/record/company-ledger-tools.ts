import type { z } from 'zod';
import type { companyDocumentDownloadInputSchema, companyDocumentListInputSchema, companyDocumentRegisterInputSchema, companyDocumentSearchInputSchema, companyDocumentUpdateInputSchema, companyDocumentUploadInputSchema, companyMetricListInputSchema, companyMetricRecordInputSchema, companyRecordAddInputSchema, companyRecordDeleteInputSchema, companyRecordListInputSchema, companyRecordUpdateInputSchema } from '../catalog/company';
import { assetBucket } from '../asset-address';
import { compareMomentsNewestFirst, momentSpanOf } from '$lib/moment';
import { dataRoomFolder, derivedPath, originalPath } from '$lib/data-room/storage-path';
import { titleNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintMatcher } from './hint-resolution';
import type { RecordContext } from './company';
import { moneySchema, type Money } from '$lib/money';
import type { MetricBasis } from '$lib/data-room/model';
import type {
	CompanyDocumentDownloadResult,
	CompanyDocumentListResult,
	CompanyDocumentPublished,
	CompanyDocumentRecallResult,
	CompanyDocumentResult,
	CompanyDocumentUploadResult,
	CompanyMetricListResult,
	CompanyMetricResult,
	CompanyRecordListResult,
	CompanyRecordResult
} from '../catalog/company';
import { InputRefused, AccessRefused, TargetMissing, RecordAlreadyHolds, refusalOfPostgresError, refusalOfServiceError } from '../refusal';

export type CompanyMetricRecordInput = z.infer<typeof companyMetricRecordInputSchema>;

export type CompanyMetricListInput = z.infer<typeof companyMetricListInputSchema>;

export type CompanyRecordAddInput = z.infer<typeof companyRecordAddInputSchema>;

export type CompanyRecordStatus = 'in_progress' | 'completed';

export type CompanyRecordUpdateInput = z.infer<typeof companyRecordUpdateInputSchema>;

export type CompanyRecordDeleteInput = z.infer<typeof companyRecordDeleteInputSchema>;

export type CompanyRecordListInput = z.infer<typeof companyRecordListInputSchema>;

export type DataRoomDocumentInput = {
	categoryCode?: string;
	date?: string;
	period?: string;
	status?: string;
	supersedesHint?: string;
	sha256?: string;
	tags?: string[];
};

export type CompanyDocumentRegisterInput = z.infer<typeof companyDocumentRegisterInputSchema>;

export type CompanyDocumentUpdateInput = z.infer<typeof companyDocumentUpdateInputSchema>;

export type CompanyDocumentListInput = z.infer<typeof companyDocumentListInputSchema>;

export type CompanyDocumentUploadInput = z.infer<typeof companyDocumentUploadInputSchema>;

export type CompanyDocumentDownloadInput = z.infer<typeof companyDocumentDownloadInputSchema>;

export type CompanyDocumentSearchInput = z.infer<typeof companyDocumentSearchInputSchema>;
export type CompanyDocumentRecallInput = {
	query?: string;
	candidates?: string[];
	changedAfter?: string;
	changedAfterID?: string;
	limit?: number;
};

const documentSearchDefaultLimit = 5;
const documentSearchLimit = 50;
const documentRecallMinimumRelevance = 0.5;
const documentListingLimit = 100;
const downloadableForTenMinutes = 10 * 60;
const sha256Pattern = /^[0-9a-f]{64}$/;
const inboxCategoryCode = 'X';

const metricColumns = 'id, metric, period, basis, amount, value, note, updated_at';
const recordColumns = 'id, category, record_date, title, detail, status, amount, attributes, updated_at';
const documentColumns =
	'id, document_number, kind, document_type, title, counterpart, language, file_path, summary, requester_id, created_at, updated_at, category_code, document_date, period, status, supersedes, sha256, tags, storage_path, size_bytes, published_from, published_at, published_by';

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
	period: string;
	basis: MetricBasis;
	amount: Money | null;
	value: number | null;
	note: string | null;
	updated_at: string;
};

type MetricUnitRow = { field: string; unit: string | null };

type RecordRow = {
	id: string;
	category: string;
	record_date: string | null;
	title: string;
	detail: string | null;
	status: CompanyRecordStatus;
	amount: Money | null;
	attributes: Record<string, RecordAttributeValue> | null;
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
	created_at: string;
	updated_at: string;
	category_code: string;
	document_date: string | null;
	period: string | null;
	status: string | null;
	supersedes: string | null;
	sha256: string | null;
	tags: string[] | null;
	storage_path: string | null;
	size_bytes: number | null;
	published_from: string | null;
	published_at: string | null;
	published_by: string | null;
};

function answeredMetric(row: MetricRow, unit: string | null): CompanyMetricResult {
	return {
		metricID: row.id,
		metric: row.metric,
		period: row.period,
		basis: row.basis,
		amount: row.amount,
		value: row.value === null ? null : Number(row.value),
		unit: row.amount ? null : unit,
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
		status: row.status,
		amount: row.amount,
		attributes: Object.entries(row.attributes ?? {}).map(([label, value]) => ({ label, value: attributeText(value) })),
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
		createdAt: row.created_at,
		updatedAt: row.updated_at,
		categoryCode: row.category_code,
		date: row.document_date,
		period: row.period,
		status: row.status,
		supersedes: row.supersedes,
		sha256: row.sha256,
		tags: row.tags ?? [],
		storagePath: row.storage_path,
		sizeBytes: row.size_bytes,
		published: publishedOf(row)
	};
}

function publishedOf(row: DocumentRow): CompanyDocumentPublished | null {
	if (!row.published_at) return null;
	return { at: row.published_at, by: row.published_by, from: row.published_from };
}

export function orNull(value: string | undefined): string | null {
	const given = value?.trim();
	return given ? given : null;
}

const onlyAnAdministratorWritesMetrics = 'only an administrator can record a company metric';
const fundingCategory = 'funding';
const onlyAnAdministratorWritesRecords = 'only an administrator can change a company record';
const onlyAColleagueWritesDocuments = 'only somebody who works here can change the document ledger';

type RecordAttributeValue = string | Money;

function decodedAttributes(attributes: string | undefined): Record<string, RecordAttributeValue> | undefined {
	const written = attributes?.trim();
	if (!written) return undefined;
	let parsed: unknown;
	try {
		parsed = JSON.parse(written);
	} catch {
		throw new InputRefused(
			'attributes must be a JSON object string of label-to-value pairs, e.g. {"round": "Seed"}'
		);
	}
	if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
		throw new InputRefused('attributes must be a JSON object string of label-to-value pairs');
	}
	const values: Record<string, RecordAttributeValue> = {};
	for (const [label, value] of Object.entries(parsed as Record<string, unknown>)) {
		values[label] = decodedAttributeValue(label, value);
	}
	return values;
}

function decodedAttributeValue(label: string, value: unknown): RecordAttributeValue {
	if (typeof value === 'string') return value;
	if (typeof value === 'number' || typeof value === 'boolean') return String(value);
	const money = moneySchema.safeParse(value);
	if (money.success) return money.data;
	throw new InputRefused(`attributes.${label} must be a string, a number, a boolean, or money like {"amount": 30000000000, "currency": "KRW"}`);
}

function attributeText(value: RecordAttributeValue): string {
	return typeof value === 'string' ? value : `${value.amount} ${value.currency}`;
}

type StoredMetricValue = { amount: Money | null; value: number | null };

function storedMetricValueOf(input: CompanyMetricRecordInput): StoredMetricValue {
	if (input.amount && input.value !== undefined) {
		throw new InputRefused('a metric is money in amount or a number in value, never both');
	}
	if (input.amount) {
		if (orNull(input.unit) !== null) throw new InputRefused('money carries its currency, not a unit');
		return { amount: input.amount, value: null };
	}
	if (input.value === undefined) throw new InputRefused('a metric records money in amount or a number in value');
	return { amount: null, value: input.value };
}

async function recordMetricUnit(context: RecordContext, metric: string, unit: string): Promise<void> {
	const { error } = await context.caller
		.from('company_field')
		.upsert({ company_id: context.companyID, kind: 'metric', field: metric, unit }, { onConflict: 'company_id,kind,field' });
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorWritesMetrics);
}

async function metricUnits(context: RecordContext, metric: string | null): Promise<Map<string, string>> {
	let query = context.caller
		.from('company_field')
		.select('field, unit')
		.eq('company_id', context.companyID)
		.eq('kind', 'metric')
		.not('unit', 'is', null);
	if (metric) query = query.eq('field', metric);
	const { data, error } = await query.returns<MetricUnitRow[]>();
	if (error) throw refusalOfPostgresError(error);
	return new Map((data ?? []).flatMap((row) => (row.unit ? [[row.field, row.unit] as const] : [])));
}

export async function companyMetricRecord(
	context: RecordContext,
	input: CompanyMetricRecordInput
): Promise<CompanyMetricResult> {
	const { metric, period } = input;
	const stored = storedMetricValueOf(input);
	const unit = orNull(input.unit);
	if (unit) await recordMetricUnit(context, metric, unit);

	const { data, error } = await context.caller
		.from('company_metric')
		.upsert(
			{
				company_id: context.companyID,
				metric,
				period,
				basis: input.basis,
				amount: stored.amount,
				value: stored.value,
				note: orNull(input.note),
				updated_at: context.now.toISOString()
			},
			{ onConflict: 'company_id,metric,period' }
		)
		.select(metricColumns)
		.single<MetricRow>();
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorWritesMetrics);
	const units = await metricUnits(context, metric);
	return answeredMetric(data, units.get(metric) ?? null);
}

function byMetricThenPeriod(left: MetricRow, right: MetricRow): number {
	if (left.metric !== right.metric) return left.metric < right.metric ? -1 : 1;
	return (momentSpanOf(left.period)?.start ?? '').localeCompare(momentSpanOf(right.period)?.start ?? '')
		|| left.period.localeCompare(right.period);
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
	if (input.fromYear !== undefined) query = query.gte('period', String(input.fromYear));
	if (input.toYear !== undefined) query = query.lt('period', String(input.toYear + 1));

	const [{ data, error }, units] = await Promise.all([query.order('metric').returns<MetricRow[]>(), metricUnits(context, metric)]);
	if (error) throw refusalOfPostgresError(error);
	const metrics = (data ?? []).toSorted(byMetricThenPeriod).map((row) => answeredMetric(row, units.get(row.metric) ?? null));
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
		.order('updated_at', { ascending: false })
		.returns<RecordRow[]>();
	if (error) throw refusalOfPostgresError(error);
	return (data ?? []).toSorted((left, right) => compareMomentsNewestFirst(left.record_date, right.record_date));
}

export async function companyRecordOfHint(
	context: RecordContext,
	hint: string
): Promise<RecordRow> {
	const asked = hint.trim();
	if (!asked) throw new InputRefused('this call names the company record it is about');
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

export async function fundingRecordOfHint(context: RecordContext, hint: string): Promise<RecordRow> {
	const record = await companyRecordOfHint(context, hint);
	if (record.category !== fundingCategory) {
		throw new InputRefused(`${record.title} is a ${record.category} record, not a funding round`);
	}
	return record;
}

export async function companyRecordAdd(
	context: RecordContext,
	input: CompanyRecordAddInput
): Promise<CompanyRecordResult> {
	const { category, title } = input;

	const { data, error } = await context.caller
		.from('company_record')
		.insert({
			company_id: context.companyID,
			category,
			record_date: orNull(input.date),
			title,
			detail: orNull(input.detail),
			status: input.status ?? 'completed',
			amount: input.amount ?? null,
			attributes: decodedAttributes(input.attributes) ?? {},
			updated_at: context.now.toISOString()
		})
		.select(recordColumns)
		.single<RecordRow>();
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorWritesRecords);
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
	if (input.detail !== undefined) change.detail = orNull(input.detail);
	if (attributes) change.attributes = attributes;
	if (input.status) change.status = input.status;
	if (input.amount) change.amount = input.amount;

	const { data, error } = await context.caller
		.from('company_record')
		.update(change)
		.eq('id', held.id)
		.select(recordColumns)
		.returns<RecordRow[]>();
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorWritesRecords);
	const written = (data ?? [])[0];
	if (!written) throw new AccessRefused(onlyAnAdministratorWritesRecords);
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
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorWritesRecords);
	if ((data ?? []).length === 0) {
		throw new AccessRefused(onlyAnAdministratorWritesRecords);
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
	const written = [row.title, row.detail ?? '', ...Object.values(row.attributes ?? {}).map(attributeText)];
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
		.order('created_at', { ascending: false })
		.returns<DocumentRow[]>();
	if (error) throw refusalOfPostgresError(error);
	return data ?? [];
}

export async function companyDocumentOfHint(
	context: RecordContext,
	hint: string
): Promise<DocumentRow> {
	const asked = hint.trim();
	if (!asked) throw new InputRefused('this call names the document it is about');
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
): Promise<CompanyDocumentResult> {
	const { documentType, title } = input;
	const kind = kindOfDocument(input.kind);
	const prefix = `${documentNumberPrefix(documentType)}-${context.now.getUTCFullYear()}-`;

	const categoryCode = input.categoryCode ?? inboxCategoryCode;
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
		...(await dataRoomChangeOf(context, input, categoryCode)),
		category_code: categoryCode
	};

	const documentNumber = kind === 'issued' ? await reservedDocumentNumber(context, prefix) : null;
	const { data, error } = await context.caller
		.from('company_document')
		.insert({ ...written, document_number: documentNumber })
		.select(documentColumns)
		.single<DocumentRow>();
	if (error) throw refusalOfPostgresError(error, onlyAColleagueWritesDocuments);
	return answeredDocument(data);
}

async function reservedDocumentNumber(context: RecordContext, prefix: string): Promise<string> {
	const { data, error } = await context.caller.rpc('reserve_document_number', {
		target_company: context.companyID,
		requested_prefix: prefix
	});
	if (error) throw refusalOfPostgresError(error, onlyAColleagueWritesDocuments);
	return String(data);
}

export async function companyDocumentUpdate(
	context: RecordContext,
	input: CompanyDocumentUpdateInput
): Promise<CompanyDocumentResult> {
	const held = await companyDocumentOfHint(context, input.documentHint ?? '');
	const change: Record<string, unknown> = await dataRoomChangeOf(context, input, held.category_code);
	if (orNull(input.title)) change.title = input.title?.trim();
	if (orNull(input.counterpart)) change.counterpart = input.counterpart?.trim();
	if (orNull(input.filePath)) change.file_path = input.filePath?.trim();
	if (orNull(input.summary)) change.summary = input.summary?.trim();
	if (Object.keys(change).length === 0) {
		throw new InputRefused('a document update names at least one thing to change');
	}

	const relocation = await relocationOf(context, held, change.category_code);
	if (relocation) change.storage_path = relocation.storagePath;
	const moves = relocation?.moves ?? [];
	await moveFiles(context, moves);
	try {
		return answeredDocument(await writtenDocument(context, held.id, change));
	} catch (refusal) {
		await putFilesBack(context, moves);
		throw refusal;
	}
}

async function writtenDocument(context: RecordContext, documentID: string, change: Record<string, unknown>): Promise<DocumentRow> {
	const { data, error } = await context.caller
		.from('company_document')
		.update(change)
		.eq('id', documentID)
		.select(documentColumns)
		.returns<DocumentRow[]>();
	if (error) throw refusalOfPostgresError(error, onlyAColleagueWritesDocuments);
	const written = (data ?? [])[0];
	if (!written) throw new AccessRefused(onlyAColleagueWritesDocuments);
	return written;
}

type FileMove = { from: string; to: string };

type Relocation = { storagePath: string; moves: FileMove[] };

async function relocationOf(context: RecordContext, held: DocumentRow, categoryCode: unknown): Promise<Relocation | null> {
	if (typeof categoryCode !== 'string' || categoryCode === held.category_code || !held.storage_path) return null;
	const fromFolder = dataRoomFolder(context.companyID, held.category_code);
	const toFolder = dataRoomFolder(context.companyID, categoryCode);
	const relocated = (path: string) => toFolder + path.slice(fromFolder.length);
	const filesOfTheDocument = derivedPath(held.storage_path, held.id, '');
	const { data, error } = await context.caller.storage
		.from(assetBucket)
		.list(fromFolder, { search: filesOfTheDocument.slice(fromFolder.length + 1), limit: 1000 });
	if (error) throw refusalOfServiceError(error);
	const moves = data
		.map((file) => `${fromFolder}/${file.name}`)
		.filter((path) => path.startsWith(filesOfTheDocument))
		.map((path) => ({ from: path, to: relocated(path) }));
	return { storagePath: relocated(held.storage_path), moves };
}

async function moveFiles(context: RecordContext, moves: FileMove[]): Promise<void> {
	const moved: FileMove[] = [];
	for (const move of moves) {
		const { error } = await context.caller.storage.from(assetBucket).move(move.from, move.to);
		if (error) {
			await putFilesBack(context, moved);
			throw refusalOfServiceError(error, `${move.from} could not move to ${move.to}: ${error.message}`);
		}
		moved.push(move);
	}
}

async function putFilesBack(context: RecordContext, moves: FileMove[]): Promise<void> {
	for (const move of [...moves].reverse()) {
		await context.caller.storage.from(assetBucket).move(move.to, move.from);
	}
}

export async function companyDocumentList(
	context: RecordContext,
	input: CompanyDocumentListInput
): Promise<CompanyDocumentListResult> {
	const documentType = orNull(input.type);
	const counterpart = orNull(input.counterpart)?.toLowerCase();
	const keyword = orNull(input.query)?.toLowerCase();
	const kept = (await documentRows(context))
		.filter((row) => !input.categoryCode || row.category_code === input.categoryCode
			|| (input.categoryCode.length === 1 && row.category_code.startsWith(input.categoryCode)))
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
	const documents = (await documentsMatching(context, input.query, input.limit, 0)).map(answeredDocument);
	return { count: documents.length, documents };
}

export async function companyDocumentRecall(
	context: RecordContext,
	input: CompanyDocumentRecallInput
): Promise<CompanyDocumentRecallResult> {
	if (input.changedAfter !== undefined) return documentsChangedAfter(context, input.changedAfter, input.changedAfterID, input.limit);
	const asked = input.query?.trim();
	if (!asked) throw new InputRefused('a document recall asks something or lists what changed');
	const limit = Math.min(Math.max(input.limit ?? documentSearchDefaultLimit, 1), documentSearchLimit);
	const matched = await documentsMatching(context, asked, limit, documentRecallMinimumRelevance);
	const nearby = await documentsAmong(context, input.candidates ?? []);
	const recalled = [...matched, ...nearby].filter((row, index, rows) => rows.findIndex((other) => other.id === row.id) === index);
	return { facts: recalled.slice(0, limit).map(recalledDocument) };
}

async function documentsAmong(context: RecordContext, candidates: string[]): Promise<DocumentRow[]> {
	if (candidates.length === 0) return [];
	const { data, error } = await context.caller.from('company_document').select(documentColumns).in('id', candidates);
	if (error) throw refusalOfPostgresError(error);
	const readable = (data ?? []) as DocumentRow[];
	return candidates.flatMap((id) => readable.filter((row) => row.id === id));
}

async function documentsChangedAfter(
	context: RecordContext,
	changedAfter: string,
	changedAfterID: string | undefined,
	limit: number | undefined
): Promise<CompanyDocumentRecallResult> {
	const after = changedAfterID
		? `updated_at.gt."${changedAfter}",and(updated_at.eq."${changedAfter}",id.gt."${changedAfterID}")`
		: `updated_at.gt."${changedAfter}"`;
	const { data, error } = await context.caller
		.from('company_document')
		.select(documentColumns)
		.or(after)
		.order('updated_at')
		.order('id')
		.limit(Math.min(Math.max(limit ?? documentListingLimit, 1), documentListingLimit));
	if (error) throw refusalOfPostgresError(error);
	const facts = ((data ?? []) as DocumentRow[]).map((row) => ({ ...recalledDocument(row), changedAt: row.updated_at }));
	return { facts, frame: recalledSentence('', null, null) };
}

async function documentsMatching(
	context: RecordContext,
	asked: string,
	limit: number | undefined,
	minimumRelevance: number
): Promise<DocumentRow[]> {
	const { data, error } = await context.caller
		.rpc('company_documents_matching', {
			target_query: asked,
			target_limit: Math.min(Math.max(limit ?? documentSearchDefaultLimit, 1), documentSearchLimit),
			minimum_relevance: minimumRelevance
		})
		.select(documentColumns);
	if (error) throw refusalOfPostgresError(error);
	if (!Array.isArray(data)) throw new Error('a document search answers a list of documents');
	return data as DocumentRow[];
}

function recalledDocument(row: DocumentRow): CompanyDocumentRecallResult['facts'][number] {
	const content = recalledSentence(row.title, row.counterpart, row.summary);
	return row.document_date ? { id: row.id, content, validAt: row.document_date } : { id: row.id, content };
}

function recalledSentence(title: string, counterpart: string | null, summary: string | null): string {
	const withCounterpart = counterpart ? ` with ${counterpart}` : '';
	const withSummary = summary ? `: ${summary}` : '';
	return `Data room document "${title}"${withCounterpart}${withSummary}`;
}

function sha256Of(sha256: string | undefined): string {
	const given = sha256?.trim().toLowerCase() ?? '';
	if (!sha256Pattern.test(given)) throw new InputRefused('a sha256 is 64 lowercase hex characters');
	return given;
}

function derivedFileNameOf(fileName: string | undefined): string | null {
	const given = orNull(fileName);
	if (given === null) return null;
	if (given === '.' || given === '..' || given.includes('/')) {
		throw new InputRefused('a derived file name is one name beside the original, never a path');
	}
	return given;
}

function tagsOf(tags: string[]): string[] {
	return tags.map((tag) => tag.trim()).filter(Boolean);
}

async function dataRoomChangeOf(
	context: RecordContext,
	input: DataRoomDocumentInput,
	categoryCode: string
): Promise<Record<string, unknown>> {
	const change: Record<string, unknown> = {};
	if (input.categoryCode !== undefined) change.category_code = input.categoryCode;
	if (orNull(input.date)) change.document_date = input.date?.trim();
	if (orNull(input.period)) change.period = input.period?.trim();
	if (orNull(input.status)) change.status = input.status?.trim();
	if (input.sha256 !== undefined) change.sha256 = sha256Of(input.sha256);
	if (input.tags !== undefined) change.tags = tagsOf(input.tags);
	if (orNull(input.supersedesHint)) {
		change.supersedes = (await supersededDocumentOfHint(context, input.supersedesHint ?? '', input.categoryCode ?? categoryCode)).id;
	}
	return change;
}

async function supersededDocumentOfHint(
	context: RecordContext,
	hint: string,
	categoryCode: string
): Promise<DocumentRow> {
	const candidates = (await documentRows(context)).filter((row) => row.category_code === categoryCode);
	return documentAmong(candidates, hint.trim());
}

export async function companyDocumentUpload(
	context: RecordContext,
	input: CompanyDocumentUploadInput
): Promise<CompanyDocumentUploadResult> {
	const held = await companyDocumentOfHint(context, input.documentHint ?? '');
	const fileName = derivedFileNameOf(input.fileName);
	const storagePath = fileName === null
		? await recordedOriginalPath(context, held, input.originalFileName)
		: derivedPath(storedOriginalOf(held), held.id, fileName);

	const { data, error } = await context.caller.storage
		.from(assetBucket)
		.createSignedUploadUrl(storagePath, { upsert: true });
	if (error) throw refusalOfServiceError(error);
	return { storagePath, uploadURL: data.signedUrl };
}

async function recordedOriginalPath(
	context: RecordContext,
	held: DocumentRow,
	originalFileName: string | undefined
): Promise<string> {
	const fileName = originalFileName?.trim();
	if (!fileName) throw new InputRefused("an upload names the original's file name, or the fileName of a file derived from it");
	const storagePath = originalPath(
		{ companyID: context.companyID, categoryCode: held.category_code, documentID: held.id },
		fileName,
		[held.title, held.document_type]
	);
	if (held.storage_path === storagePath) return storagePath;
	if (held.storage_path) {
		throw new RecordAlreadyHolds('this document already holds its original; register the new file as a document that supersedes it', 'company_document_has_original');
	}
	const { error } = await context.caller.from('company_document').update({ storage_path: storagePath }).eq('id', held.id);
	if (error) throw refusalOfPostgresError(error, onlyAColleagueWritesDocuments);
	return storagePath;
}

export async function companyDocumentDownload(
	context: RecordContext,
	input: CompanyDocumentDownloadInput
): Promise<CompanyDocumentDownloadResult> {
	const held = await companyDocumentOfHint(context, input.documentHint ?? '');
	const original = storedOriginalOf(held);
	const fileName = derivedFileNameOf(input.fileName);
	const storagePath = fileName === null ? original : derivedPath(original, held.id, fileName);

	const { data, error } = await context.caller.storage
		.from(assetBucket)
		.createSignedUrl(storagePath, downloadableForTenMinutes);
	if (error) throw refusalOfServiceError(error);
	return { storagePath, downloadURL: data.signedUrl };
}

function storedOriginalOf(held: DocumentRow): string {
	if (!held.storage_path) {
		throw new TargetMissing('this document keeps no file in the data room', 'company_document_no_file');
	}
	return held.storage_path;
}

