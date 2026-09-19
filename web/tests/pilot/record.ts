export interface RecordAccess {
	apiURL: string;
	secretKey: string;
}

export const seedCompanyID = '000000cc-0000-0000-0000-000000000001';
const companyTimeZone = 'Asia/Seoul';

type Scalar = string | number | boolean | null;
type FilterValue = Scalar | { gte?: string; lte?: string };
export type Where = Record<string, FilterValue>;

export interface Assertion {
	table: string;
	select?: string;
	where: Where;
	rowCount?: number;
	expect: Record<string, unknown>;
}

export type CleanupStep =
	| { delete: string; where: Where }
	| { update: string; where: Where; set: Record<string, Scalar> };

export interface PilotTask {
	name: string;
	instruction: string;
	assertions: Assertion[];
	cleanup: CleanupStep[];
}

export interface Judgement {
	passed: boolean;
	findings: { table: string; rowCount: number; mismatches: string[] }[];
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

export async function judge(record: RecordAccess, assertions: Assertion[]): Promise<Judgement> {
	const findings = [];
	for (const assertion of assertions) {
		const rows = await rowsOfAnyTable(record, assertion.table, assertion.where, assertion.select ?? '*');
		const expectedRowCount = assertion.rowCount ?? 1;
		const mismatches =
			rows.length === expectedRowCount
				? rows.flatMap((row) => mismatchesOf(assertion.expect, row))
				: [`expected ${expectedRowCount} row(s), found ${rows.length}`];
		findings.push({ table: assertion.table, rowCount: rows.length, mismatches });
	}
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
