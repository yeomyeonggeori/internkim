import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

type CallSiteCounts = Record<string, Record<string, number>>;
type RecordedInventory = { record: CallSiteCounts; allowed: Record<string, CallSiteCounts> };

const webRoot = join(import.meta.dir, '..', '..');
const sourceRoot = join(webRoot, 'src');
const inventoryPath = join(import.meta.dir, 'browser-supabase-call-sites.json');
const dataCall = /\.(from|rpc)\s*(?:<[^>]*>)?\s*\(/g;

function everySourceFile(directory: string): string[] {
	return readdirSync(directory)
		.sort()
		.flatMap((entry) => {
			const path = join(directory, entry);
			if (statSync(path).isDirectory()) return everySourceFile(path);
			return /\.(ts|js|svelte)$/.test(entry) ? [path] : [];
		});
}

export function isBrowserReachable(path: string): boolean {
	if (path.startsWith('src/lib/server/')) return false;
	if (/\.server\.(ts|js)$/.test(path)) return false;
	return !path.endsWith('/+server.ts');
}

function precedingToken(source: string, before: number): string {
	let index = before;
	while (index > 0 && /\s/.test(source[index - 1])) index -= 1;
	const end = index;
	while (index > 0 && /[A-Za-z0-9_$]/.test(source[index - 1])) index -= 1;
	return source.slice(index, end);
}

function isArrayConstructor(token: string): boolean {
	return /^(Array|Buffer|[A-Za-z0-9]+Array)$/.test(token);
}

function firstArgumentText(source: string, openParenthesis: number): string {
	let depth = 0;
	for (let index = openParenthesis; index < source.length; index += 1) {
		const character = source[index];
		if ('([{'.includes(character)) depth += 1;
		else if (')]}'.includes(character)) {
			depth -= 1;
			if (depth === 0) return source.slice(openParenthesis + 1, index);
		} else if (character === ',' && depth === 1) return source.slice(openParenthesis + 1, index);
	}
	return source.slice(openParenthesis + 1);
}

export function dataCallsIn(source: string): string[] {
	const calls: string[] = [];
	for (const found of source.matchAll(dataCall)) {
		if (isArrayConstructor(precedingToken(source, found.index))) continue;
		const argument = firstArgumentText(source, found.index + found[0].length - 1)
			.replace(/\s+/g, ' ')
			.trim();
		calls.push(`${found[1]}(${argument})`);
	}
	return calls;
}

export function browserSupabaseCallSites(): CallSiteCounts {
	const callSites: CallSiteCounts = {};
	for (const absolutePath of everySourceFile(sourceRoot)) {
		const path = relative(webRoot, absolutePath);
		if (!isBrowserReachable(path)) continue;
		for (const call of dataCallsIn(readFileSync(absolutePath, 'utf8'))) {
			callSites[path] ??= {};
			callSites[path][call] = (callSites[path][call] ?? 0) + 1;
		}
	}
	return callSites;
}

function isCallSiteCounts(value: unknown): value is CallSiteCounts {
	if (typeof value !== 'object' || value === null) return false;
	return Object.values(value).every(
		(calls) =>
			typeof calls === 'object' &&
			calls !== null &&
			Object.values(calls).every((count) => Number.isInteger(count))
	);
}

function isRecordedInventory(value: unknown): value is RecordedInventory {
	if (typeof value !== 'object' || value === null) return false;
	const { record, allowed } = value as Record<string, unknown>;
	if (!isCallSiteCounts(record)) return false;
	if (typeof allowed !== 'object' || allowed === null) return false;
	return Object.values(allowed).every(isCallSiteCounts);
}

export function recordedInventory(): RecordedInventory {
	const parsed: unknown = JSON.parse(readFileSync(inventoryPath, 'utf8'));
	if (!isRecordedInventory(parsed)) {
		throw new Error(`${inventoryPath} is not { record, allowed } of file to call to count`);
	}
	return parsed;
}

export function recordedCallSites(): CallSiteCounts {
	const { record, allowed } = recordedInventory();
	const merged: CallSiteCounts = {};
	for (const bucket of [record, ...Object.values(allowed)]) {
		for (const [path, calls] of Object.entries(bucket)) {
			merged[path] ??= {};
			for (const [call, count] of Object.entries(calls)) {
				if (merged[path][call] !== undefined) {
					throw new Error(`${inventoryPath}: ${path} ${call} is recorded in more than one place`);
				}
				merged[path][call] = count;
			}
		}
	}
	return merged;
}
