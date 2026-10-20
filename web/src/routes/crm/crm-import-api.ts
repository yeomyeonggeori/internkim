import type {
	CRMRecordImportKind,
	CRMRecordImportOutcome,
	CRMRecordImportProblem
} from '$lib/crm/crm-record-import';
import { callTool } from './crm-public-api';
import type { CRMImportRow } from '$lib/crm/crm-import-mapping';

export type CRMImportedRow = {
	line: number;
	outcome: CRMRecordImportOutcome;
	duplicateOfID: string;
	recordID: string;
	problems: CRMRecordImportProblem[];
};

export type CRMImportResult = {
	dryRun: boolean;
	written: boolean;
	created: number;
	updated: number;
	skipped: number;
	refused: number;
	rows: CRMImportedRow[];
};

export async function importCRMRecords(
	kind: CRMRecordImportKind,
	rows: CRMImportRow[],
	dryRun: boolean
): Promise<CRMImportResult> {
	return callTool<CRMImportResult>('crm_record_import', { kind, rows, dryRun });
}
