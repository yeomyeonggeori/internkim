export type Soul = {
	values: string[];
	boundaries: string[];
	workingStyle: string[];
	register: string;
	traits: string[];
	languageDefault: string;
	matchRequester: boolean;
};

type LineField = 'values' | 'boundaries' | 'workingStyle';

export type SoulChange =
	| { field: LineField; added?: string; removed?: string }
	| { field: 'register'; added?: string; removed?: string }
	| { field: 'traits'; added?: string[]; removed?: string[] }
	| { field: 'language'; added?: Soul; removed?: Soul };

const lineFields: LineField[] = ['values', 'boundaries', 'workingStyle'];

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringsOf(value: unknown): string[] {
	return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '') : [];
}

function stringOf(value: unknown): string {
	return typeof value === 'string' ? value : '';
}

export function soulOf(document: unknown): Soul {
	const record = isRecord(document) ? document : {};
	const tone = isRecord(record.tone) ? record.tone : {};
	const language = isRecord(record.language) ? record.language : {};
	return {
		values: stringsOf(record.values),
		boundaries: stringsOf(record.boundaries),
		workingStyle: stringsOf(record.workingStyle),
		register: stringOf(tone.register),
		traits: stringsOf(tone.traits),
		languageDefault: stringOf(language.default),
		matchRequester: language.matchRequester === true
	};
}

export function soulDocumentOf(soul: Soul): Record<string, unknown> {
	const linesOf = (lines: string[]) => [...new Set(lines.map((line) => line.trim()).filter((line) => line !== ''))];
	const tone = soul.register || soul.traits.length ? { ...(soul.register ? { register: soul.register } : {}), ...(soul.traits.length ? { traits: soul.traits } : {}) } : undefined;
	const language = soul.languageDefault || soul.matchRequester ? { ...(soul.languageDefault ? { default: soul.languageDefault } : {}), ...(soul.matchRequester ? { matchRequester: true } : {}) } : undefined;
	return {
		schemaVersion: 1,
		...Object.fromEntries(lineFields.map((field) => [field, linesOf(soul[field])]).filter(([, lines]) => lines.length > 0)),
		...(tone ? { tone } : {}),
		...(language ? { language } : {})
	};
}

function lineChanges(field: LineField, before: string[], after: string[]): SoulChange[] {
	const removed = before.filter((line) => !after.includes(line)).map((line): SoulChange => ({ field, removed: line }));
	const added = after.filter((line) => !before.includes(line)).map((line): SoulChange => ({ field, added: line }));
	return [...removed, ...added];
}

function isSameLanguage(before: Soul, after: Soul): boolean {
	return before.languageDefault === after.languageDefault && before.matchRequester === after.matchRequester;
}

export function soulChangesBetween(before: Soul, after: Soul): SoulChange[] {
	const changes = lineFields.flatMap((field) => lineChanges(field, before[field], after[field]));
	if (before.register !== after.register) changes.push({ field: 'register', removed: before.register || undefined, added: after.register || undefined });
	if (before.traits.join('\n') !== after.traits.join('\n')) changes.push({ field: 'traits', removed: before.traits, added: after.traits });
	if (!isSameLanguage(before, after)) changes.push({ field: 'language', removed: before, added: after });
	return changes;
}
