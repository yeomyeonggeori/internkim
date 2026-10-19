import { crmImportMaxFileBytes, readCRMImportBytes } from '$lib/crm/crm-import-csv';
import {
	crmImportFields,
	importedRows,
	missingRequiredFields,
	type CRMImportMapping
} from '$lib/crm/crm-import-mapping';
import { crmImportValueLabels } from '../../../../routes/crm/crm-import-headers';
import type { CarriedWorkspaceFile } from '../carried-files';
import type { RecordContext } from './company';
import { crmRecordImport } from './crm-import-tools';
import type { PreviewedTarget } from './preview';
import {
	crmRecordImportRowsSchema,
	type CRMRecordImportInput,
	type CRMRecordImportRecords,
	type CRMRecordImportResult
} from '../catalog/crm';

export type CRMImportFile = { path: string; filename: string; records: CRMRecordImportRecords };

const megabyte = 1024 * 1024;
const refusedLinesNamed = 20;

function namedPath(input: CRMRecordImportInput): string | undefined {
	return input.attachments?.[0];
}

function carriedFileOf(path: string, files: CarriedWorkspaceFile[]): CarriedWorkspaceFile | undefined {
	return files.find((file) => file.workspacePath === path);
}

function decodedSize(file: CarriedWorkspaceFile): number {
	const padding = file.contentBase64.endsWith('==') ? 2 : file.contentBase64.endsWith('=') ? 1 : 0;
	return (file.contentBase64.length * 3) / 4 - padding;
}

function bytesOf(file: CarriedWorkspaceFile): Uint8Array {
	const size = decodedSize(file);
	if (size > crmImportMaxFileBytes) {
		throw new Error(`${file.workspacePath} is ${(size / megabyte).toFixed(1)} MB, and an import reads at most ${crmImportMaxFileBytes / megabyte} MB`);
	}
	return Uint8Array.from(atob(file.contentBase64), (character) => character.charCodeAt(0));
}

function mappingOf(input: CRMRecordImportInput, headers: string[]): CRMImportMapping {
	const columns = Object.entries(input.columns ?? {});
	const missingHeaders = columns.filter(([, header]) => !headers.includes(header?.trim() ?? ''));
	if (missingHeaders.length > 0) {
		throw new Error(`columns names ${missingHeaders.map(([, header]) => `"${header}"`).join(', ')}, which the file does not have; its columns are ${headers.map((header) => `"${header}"`).join(', ')}`);
	}
	const fields = crmImportFields[input.kind];
	return headers.map((header) => fields.find((field) => input.columns?.[field]?.trim() === header) ?? null);
}

function refuseUnmappedRequiredFields(input: CRMRecordImportInput, mapping: CRMImportMapping): void {
	const missing = missingRequiredFields(input.kind, mapping);
	if (missing.length > 0) throw new Error(`a ${input.kind} import needs a column for ${missing.join(' and ')} in columns`);
}

function refuseAFileWithRows(input: CRMRecordImportInput): void {
	if (namedPath(input) && input.rows) throw new Error('name the file in attachments or send rows, not both');
}

export async function crmImportRecordsOf(
	input: CRMRecordImportInput,
	files: CarriedWorkspaceFile[]
): Promise<CRMRecordImportRecords> {
	refuseAFileWithRows(input);
	const path = namedPath(input);
	if (!path) {
		if (!input.rows) throw new Error('name the file to import in attachments, or send its rows');
		return { kind: input.kind, rows: input.rows, dryRun: input.dryRun };
	}
	const file = await crmImportFileOf(input, files);
	if (!file) throw new Error(`${path} is read only when the company host carries its content with the call, and this call carried none`);
	return file.records;
}

export async function crmImportFileOf(
	input: CRMRecordImportInput,
	files: CarriedWorkspaceFile[]
): Promise<CRMImportFile | null> {
	const path = namedPath(input);
	if (!path) return null;
	const carried = carriedFileOf(path, files);
	if (!carried) return null;
	const sheet = await readCRMImportBytes(carried.filename ?? path, bytesOf(carried));
	const mapping = mappingOf(input, sheet.headers);
	refuseUnmappedRequiredFields(input, mapping);
	const rows = importedRows(mapping, sheet.lines, crmImportValueLabels).map((row) => ({
		...row,
		...(input.onDuplicate ? { onDuplicate: input.onDuplicate } : {})
	}));
	if (rows.length === 0) throw new Error(`${path} has a header and no record under it`);
	return {
		path,
		filename: sheet.fileName,
		records: { kind: input.kind, rows: crmRecordImportRowsSchema.parse(rows), dryRun: input.dryRun }
	};
}

function refusedLines(checked: CRMRecordImportResult): string {
	const refused = checked.rows.filter((row) => row.outcome === 'refused');
	const named = refused.slice(0, refusedLinesNamed).map((row) => `line ${row.line}: ${row.problems.join(', ')}`);
	const unnamed = refused.length - named.length;
	return [...named, ...(unnamed > 0 ? [`and ${unnamed} more lines`] : [])].join('; ');
}

export async function crmRecordImportPreview(
	context: RecordContext,
	input: CRMRecordImportInput,
	files: CarriedWorkspaceFile[]
): Promise<PreviewedTarget | null> {
	refuseAFileWithRows(input);
	const file = await crmImportFileOf(input, files);
	if (!file) return null;
	const checked = await crmRecordImport(context, { ...file.records, dryRun: true });
	if (checked.refused > 0) {
		throw new Error(`${file.filename} has ${checked.refused} lines that cannot be imported, so nothing would be written: ${refusedLines(checked)}`);
	}
	return {
		inputField: 'attachments',
		ids: [file.path],
		title: file.filename,
		preview: `creates ${checked.created}, updates ${checked.updated}, skips ${checked.skipped} already held`
	};
}
