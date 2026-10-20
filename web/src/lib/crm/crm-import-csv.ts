export const crmImportMaxFileBytes = 5 * 1024 * 1024;

export type CRMImportLine = {
	line: number;
	cells: string[];
};

export type CRMImportSheet = {
	fileName: string;
	headers: string[];
	lines: CRMImportLine[];
};

function decodedText(bytes: AllowSharedBufferSource): string {
	try {
		return new TextDecoder('utf-8', { fatal: true }).decode(bytes);
	} catch (error) {
		if (!(error instanceof TypeError)) throw error;
		return new TextDecoder('euc-kr').decode(bytes);
	}
}

function isBlank(row: string[]): boolean {
	return row.every((cell) => cell.trim() === '');
}

function textCells(row: unknown[]): string[] {
	return row.map((cell) => (cell === null || cell === undefined ? '' : String(cell)));
}

export async function readCRMImportFile(file: File): Promise<CRMImportSheet> {
	return readCRMImportBytes(file.name, await file.arrayBuffer());
}

export async function readCRMImportBytes(fileName: string, bytes: AllowSharedBufferSource): Promise<CRMImportSheet> {
	const spreadsheet = await import('xlsx');
	const workbook = spreadsheet.read(decodedText(bytes), {
		type: 'string',
		raw: true,
		dense: true
	});
	const sheet = workbook.Sheets[workbook.SheetNames[0]];
	const grid = spreadsheet.utils
		.sheet_to_json<unknown[]>(sheet, { header: 1, raw: false, defval: '', blankrows: true })
		.map(textCells);
	const [headers = [], ...rows] = grid;
	return {
		fileName,
		headers: headers.map((header) => header.trim()),
		lines: rows
			.map((cells, index) => ({ line: index + 2, cells }))
			.filter((line) => !isBlank(line.cells))
	};
}
