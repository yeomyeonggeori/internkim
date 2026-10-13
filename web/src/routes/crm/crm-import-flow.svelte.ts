import type {
	CRMRecordImportDuplicateChoice,
	CRMRecordImportKind,
	CRMRecordImportOutcome
} from '$lib/crm/crm-record-import';
import { importCRMRecords, type CRMImportedRow, type CRMImportResult } from './crm-import-api';
import { readCRMImportFile, type CRMImportSheet } from './crm-import-csv';
import {
	crmImportFields,
	importedRows,
	missingRequiredFields,
	suggestedMapping,
	withField,
	type CRMImportField,
	type CRMImportMapping,
	type CRMImportRow,
	type CRMImportValueLabels
} from './crm-import-mapping';

export type CRMImportStep = 'upload' | 'mapping' | 'preview';
export const crmImportFilters = ['all', 'refused', 'duplicate'] as const;
export type CRMImportFilter = (typeof crmImportFilters)[number];

export type CRMImportPreviewRow = {
	line: number;
	name: string;
	outcome: CRMRecordImportOutcome;
	isDuplicate: boolean;
	problems: CRMImportedRow['problems'];
};

type CRMImportFlowOptions = {
	kind: CRMRecordImportKind;
	fieldNames: (field: CRMImportField) => string[];
	valueLabels: CRMImportValueLabels;
};

export class CRMImportFlow {
	step = $state<CRMImportStep>('upload');
	sheet = $state<CRMImportSheet | null>(null);
	mapping = $state<CRMImportMapping>([]);
	checked = $state<CRMImportResult | null>(null);
	choices = $state<Record<number, CRMRecordImportDuplicateChoice>>({});
	filter = $state<CRMImportFilter>('all');
	isBusy = $state(false);

	readonly kind: CRMRecordImportKind;
	readonly fields: readonly CRMImportField[];
	private readonly options: CRMImportFlowOptions;
	private rows: CRMImportRow[] = [];

	constructor(options: CRMImportFlowOptions) {
		this.options = options;
		this.kind = options.kind;
		this.fields = crmImportFields[options.kind];
	}

	get missingFields(): CRMImportField[] {
		return missingRequiredFields(this.kind, this.mapping);
	}

	get previewRows(): CRMImportPreviewRow[] {
		const rowsByLine = new Map(this.rows.map((row) => [row.line, row]));
		return (this.checked?.rows ?? []).map((row) => ({
			line: row.line,
			name: rowsByLine.get(row.line)?.name ?? '',
			outcome: this.outcomeOf(row),
			isDuplicate: row.duplicateOfID !== '',
			problems: row.problems
		}));
	}

	get visibleRows(): CRMImportPreviewRow[] {
		if (this.filter === 'refused') return this.previewRows.filter((row) => row.outcome === 'refused');
		if (this.filter === 'duplicate') return this.previewRows.filter((row) => row.isDuplicate);
		return this.previewRows;
	}

	count(outcome: CRMRecordImportOutcome): number {
		return this.previewRows.filter((row) => row.outcome === outcome).length;
	}

	get savableCount(): number {
		return this.count('create') + this.count('update');
	}

	async readFile(file: File): Promise<void> {
		this.isBusy = true;
		try {
			const sheet = await readCRMImportFile(file);
			this.sheet = sheet;
			this.mapping = suggestedMapping(sheet.headers, this.fields, this.options.fieldNames);
			this.step = 'mapping';
		} finally {
			this.isBusy = false;
		}
	}

	chooseField(column: number, field: CRMImportField | null): void {
		this.mapping = withField(this.mapping, column, field);
	}

	choose(line: number, choice: CRMRecordImportDuplicateChoice): void {
		this.choices = { ...this.choices, [line]: choice };
	}

	choiceOf(line: number): CRMRecordImportDuplicateChoice {
		return this.choices[line] ?? 'skip';
	}

	async check(): Promise<void> {
		if (!this.sheet) return;
		this.rows = importedRows(this.mapping, this.sheet.lines, this.options.valueLabels);
		this.isBusy = true;
		try {
			this.checked = await importCRMRecords(this.kind, this.rows, true);
			this.choices = {};
			this.filter = 'all';
			this.step = 'preview';
		} finally {
			this.isBusy = false;
		}
	}

	async save(): Promise<CRMImportResult> {
		const savable = new Set(
			this.previewRows.filter((row) => row.outcome === 'create' || row.outcome === 'update').map((row) => row.line)
		);
		const rows = this.rows
			.filter((row) => savable.has(row.line))
			.map((row) => ({ ...row, onDuplicate: this.choiceOf(row.line) }));
		this.isBusy = true;
		try {
			const saved = await importCRMRecords(this.kind, rows, false);
			if (!saved.written) this.checked = saved;
			return saved;
		} finally {
			this.isBusy = false;
		}
	}

	back(): void {
		this.step = this.step === 'preview' ? 'mapping' : 'upload';
	}

	private outcomeOf(row: CRMImportedRow): CRMRecordImportOutcome {
		if (row.outcome === 'refused' || row.duplicateOfID === '') return row.outcome;
		return this.choiceOf(row.line);
	}
}
