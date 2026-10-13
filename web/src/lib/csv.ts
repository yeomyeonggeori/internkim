const byteOrderMark = '﻿';
const csvLineBreak = '\r\n';
const spreadsheetFormulaStart = /^[=+\-@\t\r]/;

function csvCell(value: string): string {
	const safe = spreadsheetFormulaStart.test(value) ? `'${value}` : value;
	return /[",\r\n]/.test(safe) ? `"${safe.replaceAll('"', '""')}"` : safe;
}

export function csvDocument(rows: readonly (readonly string[])[]): string {
	const lines = rows.map((row) => row.map(csvCell).join(','));
	return `${byteOrderMark}${lines.join(csvLineBreak)}${csvLineBreak}`;
}
