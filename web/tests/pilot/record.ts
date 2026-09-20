import { readFileSync, readdirSync } from 'node:fs';
import { extname, join } from 'node:path';
import type { DeliveredFile, HarnessOutcome } from './arms/arm';

export interface RecordAccess {
	apiURL: string;
	secretKey: string;
}

export const seedCompanyID = '000000cc-0000-0000-0000-000000000001';
const companyTimeZone = 'Asia/Seoul';

type Scalar = string | number | boolean | null;
type FilterValue = Scalar | { gte?: string; lte?: string };
export type Where = Record<string, FilterValue>;

export interface RecordAssertion {
	table: string;
	select?: string;
	where: Where;
	rowCount?: number;
	expect: Record<string, unknown>;
}

export interface DeliveredFileAssertion {
	deliveredFile: {
		filenameEndsWith: string;
		contentType?: string;
		minBytes?: number;
	};
}

export interface ReplyMentionsAssertion {
	replyMentions: string[];
}

export interface WaitingForAnswerAssertion {
	waitingForAnswer: { mentions: string[] };
}

export type Assertion = RecordAssertion | DeliveredFileAssertion | ReplyMentionsAssertion | WaitingForAnswerAssertion;

export type CleanupStep =
	| { delete: string; where: Where }
	| { update: string; where: Where; set: Record<string, unknown> };

export interface PilotTask {
	name: string;
	requesterEmail?: string;
	instruction: string;
	assertions: Assertion[];
	cleanup: CleanupStep[];
}

export interface Finding {
	subject: string;
	rowCount: number;
	mismatches: string[];
	notes: string[];
}

export interface Judgement {
	passed: boolean;
	findings: Finding[];
}

export type JudgedOutcome = Pick<HarnessOutcome, 'status' | 'reply' | 'deliveredFiles'>;

export function isRecordAssertion(assertion: Assertion): assertion is RecordAssertion {
	return 'table' in assertion;
}

export function isDeliveredFileAssertion(assertion: Assertion): assertion is DeliveredFileAssertion {
	return 'deliveredFile' in assertion;
}

export function isReplyMentionsAssertion(assertion: Assertion): assertion is ReplyMentionsAssertion {
	return 'replyMentions' in assertion;
}

export function loadTasks(directory: string, only?: string): PilotTask[] {
	return readdirSync(directory)
		.filter((name) => name.endsWith('.json'))
		.sort()
		.map((name) => JSON.parse(readFileSync(join(directory, name), 'utf8')) as PilotTask)
		.filter((task) => !only || task.name === only);
}

export function localRecord(): RecordAccess {
	const status = Bun.spawnSync(['supabase', 'status', '-o', 'env'], { stdout: 'pipe', stderr: 'pipe' });
	const lines = new TextDecoder().decode(status.stdout).split('\n');
	const valueOf = (name: string) =>
		lines
			.find((line) => line.startsWith(`${name}=`))
			?.slice(name.length + 1)
			.replace(/^"|"$/g, '');
	const apiURL = valueOf('API_URL');
	const secretKey = valueOf('SECRET_KEY') ?? valueOf('SERVICE_ROLE_KEY');
	if (!apiURL || !secretKey) throw new Error('the local record is not running: supabase status names no API_URL and SECRET_KEY');
	return { apiURL, secretKey };
}

function tableRequest(record: RecordAccess, table: string, where: Where, select?: string): { url: string; headers: Record<string, string> } {
	const parameters = new URLSearchParams();
	if (select) parameters.set('select', select);
	parameters.set('company_id', `eq.${seedCompanyID}`);
	for (const [column, value] of Object.entries(where)) {
		if (value !== null && typeof value === 'object') {
			if (value.gte !== undefined) parameters.append(column, `gte.${value.gte}`);
			if (value.lte !== undefined) parameters.append(column, `lte.${value.lte}`);
			continue;
		}
		parameters.set(column, `eq.${value}`);
	}
	return {
		url: `${record.apiURL}/rest/v1/${table}?${parameters}`,
		headers: {
			apikey: record.secretKey,
			Authorization: `Bearer ${record.secretKey}`,
			'Content-Type': 'application/json',
		},
	};
}

async function rowsOf(record: RecordAccess, table: string, where: Where, select: string): Promise<Record<string, unknown>[]> {
	const request = tableRequest(record, table, where, select);
	const answer = await fetch(request.url, { headers: request.headers });
	if (!answer.ok) throw new Error(`${table}: the record answered ${answer.status} ${await answer.text()}`);
	return (await answer.json()) as Record<string, unknown>[];
}

const tablesWithoutCompanyColumn = new Set(['leave', 'task_participant']);

function withoutCompanyFilter(url: string): string {
	const parsed = new URL(url);
	parsed.searchParams.delete('company_id');
	return parsed.toString();
}

async function rowsOfAnyTable(record: RecordAccess, table: string, where: Where, select: string): Promise<Record<string, unknown>[]> {
	if (!tablesWithoutCompanyColumn.has(table)) return rowsOf(record, table, where, select);
	const request = tableRequest(record, table, where, select);
	const answer = await fetch(withoutCompanyFilter(request.url), { headers: request.headers });
	if (!answer.ok) throw new Error(`${table}: the record answered ${answer.status} ${await answer.text()}`);
	return (await answer.json()) as Record<string, unknown>[];
}

function dayInCompanyZone(instant: string): string {
	return new Intl.DateTimeFormat('en-CA', { timeZone: companyTimeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(instant));
}

function isPredicate(expected: unknown): expected is Record<string, unknown> {
	if (expected === null || typeof expected !== 'object' || Array.isArray(expected)) return false;
	const keys = Object.keys(expected);
	return keys.length === 1 && ['dayInZone', 'instant', 'contains'].includes(keys[0]);
}

export function matches(expected: unknown, actual: unknown): boolean {
	if (isPredicate(expected)) {
		if ('dayInZone' in expected) return typeof actual === 'string' && dayInCompanyZone(actual) === expected.dayInZone;
		if ('instant' in expected) return typeof actual === 'string' && Date.parse(actual) === Date.parse(String(expected.instant));
		return Array.isArray(actual) && actual.some((element) => matches(expected.contains, element));
	}
	if (Array.isArray(expected)) {
		return Array.isArray(actual) && expected.every((element) => actual.some((candidate) => matches(element, candidate)));
	}
	if (expected !== null && typeof expected === 'object') {
		if (actual === null || typeof actual !== 'object') return false;
		return Object.entries(expected).every(([key, value]) => matches(value, (actual as Record<string, unknown>)[key]));
	}
	if (typeof expected === 'number') return Number(actual) === expected;
	return expected === actual;
}

function mismatchesOf(expected: Record<string, unknown>, row: Record<string, unknown>): string[] {
	return Object.entries(expected)
		.filter(([column, value]) => !matches(value, row[column]))
		.map(([column, value]) => `${column}: expected ${JSON.stringify(value)}, found ${JSON.stringify(row[column])}`);
}

async function recordFinding(record: RecordAccess, assertion: RecordAssertion): Promise<Finding> {
	const rows = await rowsOfAnyTable(record, assertion.table, assertion.where, assertion.select ?? '*');
	const expectedRowCount = assertion.rowCount ?? 1;
	const mismatches =
		rows.length === expectedRowCount
			? rows.flatMap((row) => mismatchesOf(assertion.expect, row))
			: [`expected ${expectedRowCount} row(s), found ${rows.length}`];
	return { subject: assertion.table, rowCount: rows.length, mismatches, notes: [] };
}

const zipContainerExtensions = new Set(['.docx', '.pptx', '.xlsx']);

function isZipContainerFilename(filename: string): boolean {
	return zipContainerExtensions.has(extname(filename).toLowerCase());
}

function deliveredFileMismatches(expected: DeliveredFileAssertion['deliveredFile'], file: DeliveredFile): string[] {
	const mismatches: string[] = [];
	if (expected.contentType !== undefined && file.contentType !== expected.contentType) {
		mismatches.push(`contentType: expected ${expected.contentType}, found ${file.contentType || 'nothing'}`);
	}
	if (expected.minBytes !== undefined && file.sizeBytes < expected.minBytes) {
		mismatches.push(`sizeBytes: expected at least ${expected.minBytes}, found ${file.sizeBytes}`);
	}
	if (isZipContainerFilename(file.filename) && file.isZipContainer === false) {
		mismatches.push(`${file.filename} is not a readable zip container`);
	}
	return mismatches;
}

function deliveredFileNotes(file: DeliveredFile): string[] {
	if (!isZipContainerFilename(file.filename) || file.isZipContainer !== null) return [];
	return [`${file.filename}: the arm could not read the bytes, so only the attachment metadata was checked`];
}

export function deliveredFileFinding(assertion: DeliveredFileAssertion, deliveredFiles: DeliveredFile[]): Finding {
	const suffix = assertion.deliveredFile.filenameEndsWith.toLowerCase();
	const candidates = deliveredFiles.filter((file) => file.filename.toLowerCase().endsWith(suffix));
	if (candidates.length === 0) {
		return {
			subject: 'deliveredFile',
			rowCount: 0,
			mismatches: [`no delivered file ends with ${assertion.deliveredFile.filenameEndsWith}`],
			notes: [],
		};
	}
	const judged = candidates.map((file) => ({ file, mismatches: deliveredFileMismatches(assertion.deliveredFile, file) }));
	const accepted = judged.find((candidate) => candidate.mismatches.length === 0) ?? judged[0];
	return {
		subject: 'deliveredFile',
		rowCount: candidates.length,
		mismatches: accepted.mismatches,
		notes: accepted.mismatches.length === 0 ? deliveredFileNotes(accepted.file) : [],
	};
}

export function replyMentionsFinding(assertion: ReplyMentionsAssertion, reply: string): Finding {
	const missing = assertion.replyMentions.filter((value) => !reply.includes(value));
	return {
		subject: 'replyMentions',
		rowCount: assertion.replyMentions.length - missing.length,
		mismatches: missing.map((value) => `the reply does not mention ${JSON.stringify(value)}`),
		notes: [],
	};
}

const statusesThatCanCarryAQuestion: JudgedOutcome['status'][] = ['waiting_user_input', 'completed'];

export function waitingForAnswerFinding(assertion: WaitingForAnswerAssertion, outcome: JudgedOutcome): Finding {
	const mismatches: string[] = [];
	if (!statusesThatCanCarryAQuestion.includes(outcome.status)) {
		mismatches.push(`expected the run to stop and ask, found ${outcome.status}`);
	}
	if (!assertion.waitingForAnswer.mentions.some((value) => outcome.reply.includes(value))) {
		mismatches.push(`the question mentions none of ${JSON.stringify(assertion.waitingForAnswer.mentions)}`);
	}
	const askedWithoutPausing = mismatches.length === 0 && outcome.status === 'completed';
	return {
		subject: 'waitingForAnswer',
		rowCount: mismatches.length === 0 ? 1 : 0,
		mismatches,
		notes: askedWithoutPausing ? ['the arm has no paused state, so the question was read from the final reply'] : [],
	};
}

async function findingOf(record: RecordAccess, assertion: Assertion, outcome: JudgedOutcome): Promise<Finding> {
	if (isRecordAssertion(assertion)) return recordFinding(record, assertion);
	if (isDeliveredFileAssertion(assertion)) return deliveredFileFinding(assertion, outcome.deliveredFiles);
	if (isReplyMentionsAssertion(assertion)) return replyMentionsFinding(assertion, outcome.reply);
	return waitingForAnswerFinding(assertion, outcome);
}

export async function judge(record: RecordAccess, assertions: Assertion[], outcome: JudgedOutcome): Promise<Judgement> {
	const findings: Finding[] = [];
	for (const assertion of assertions) findings.push(await findingOf(record, assertion, outcome));
	return { passed: findings.every((finding) => finding.mismatches.length === 0), findings };
}

export async function cleanUp(record: RecordAccess, steps: CleanupStep[]): Promise<void> {
	for (const step of steps) {
		const table = 'delete' in step ? step.delete : step.update;
		const request = tableRequest(record, table, step.where);
		const url = tablesWithoutCompanyColumn.has(table) ? withoutCompanyFilter(request.url) : request.url;
		const answer = await fetch(url, {
			method: 'delete' in step ? 'DELETE' : 'PATCH',
			headers: { ...request.headers, Prefer: 'return=minimal' },
			body: 'set' in step ? JSON.stringify(step.set) : undefined,
		});
		if (!answer.ok) throw new Error(`cleanup ${table}: the record answered ${answer.status} ${await answer.text()}`);
	}
}
