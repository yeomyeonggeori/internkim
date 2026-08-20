import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { attendanceText } from '../../../src/routes/attendance/text';
import {
	balanceOwnLedgerOperations,
	isBalanceOwnLedgerOperation,
	ledgerOperationLabel,
	requestDerivedLedgerOperations
} from '../../../src/routes/attendance/leave/leave-ledger-operations';

const schemaPath = new URL(
	'../../../../internal/admind/attendance_leave_ledger_schema.go',
	import.meta.url
);

function operationKindsFromSchema(): string[] {
	const schema = readFileSync(schemaPath, 'utf8');
	const table = schema.slice(schema.indexOf('CREATE TABLE IF NOT EXISTS attendance_leave_operations'));
	const checkClause = table.match(/kind IN \(([^)]*)\)/);
	if (!checkClause) {
		throw new Error('operation kind CHECK constraint not found in the Go schema');
	}
	return [...checkClause[1].matchAll(/'([^']+)'/g)].map((match) => match[1]);
}

describe('leave ledger operation vocabulary', () => {
	test('covers every operation kind the schema allows', () => {
		const known = new Set<string>([
			...requestDerivedLedgerOperations,
			...balanceOwnLedgerOperations
		]);
		const missing = operationKindsFromSchema().filter((kind) => !known.has(kind));

		expect(missing).toEqual([]);
	});

	test('labels every operation kind the schema allows', () => {
		const unlabelled = operationKindsFromSchema().filter(
			(kind) => ledgerOperationLabel(attendanceText.ko.leave, kind) === attendanceText.ko.leave.operationOther
		);

		expect(unlabelled).toEqual([]);
	});

	test('keeps request-derived entries out of the history and balance events in', () => {
		expect(isBalanceOwnLedgerOperation('reserve')).toBe(false);
		expect(isBalanceOwnLedgerOperation('release')).toBe(false);
		expect(isBalanceOwnLedgerOperation('use')).toBe(false);
		expect(isBalanceOwnLedgerOperation('untrackedUse')).toBe(false);

		expect(isBalanceOwnLedgerOperation('grant')).toBe(true);
		expect(isBalanceOwnLedgerOperation('expire')).toBe(true);
		expect(isBalanceOwnLedgerOperation('carryover')).toBe(true);
		expect(isBalanceOwnLedgerOperation('legalCorrection')).toBe(true);
		expect(isBalanceOwnLedgerOperation('adjustment')).toBe(true);
	});

	test('shows an unfamiliar kind rather than dropping it', () => {
		expect(isBalanceOwnLedgerOperation('somethingAddedLater')).toBe(true);
	});
});
