import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

type CallSiteCounts = Record<string, Record<string, number>>;
type RecordedInventory = { record: CallSiteCounts; allowed: Record<string, CallSiteCounts> };

const webRoot = join(import.meta.dir, '..', '..');
const sourceRoot = join(webRoot, 'src');
const inventoryPath = join(import.meta.dir, 'browser-supabase-call-sites.json');
const theBrowserClient = 'supabase()';
const nameBoundToTheClient = /\b([A-Za-z_$][\w$]*)\s*(?::\s*SupabaseClient\s*)?=\s*supabase\(\)(?!\s*\.)/g;
const memberChain = /^\s*\.\s*([A-Za-z_$][\w$]*(?:\s*\.\s*[A-Za-z_$][\w$]*)*)\s*(?:<[^>]*>)?\s*(\()?/;
const namedByTheirFirstArgument = new Set(['from', 'rpc', 'invoke']);

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

function namesOfTheClient(source: string): string[] {
	return [...new Set([theBrowserClient, ...[...source.matchAll(nameBoundToTheClient)].map((bound) => bound[1])])];
}

function referencesTo(source: string, name: string): number[] {
	const pattern = new RegExp(`(?<![\\w$.])${name.replace(/[()$]/g, '\\$&')}`, 'g');
	return [...source.matchAll(pattern)].map((found) => found.index + name.length);
}

function accessAt(source: string, after: number): string | null {
	const chain = memberChain.exec(source.slice(after));
	if (!chain) return null;
	const members = chain[1].split(/\s*\.\s*/).join('.');
	const last = members.split('.').at(-1) ?? '';
	if (!chain[2] || !namedByTheirFirstArgument.has(last)) return members;
	const argument = firstArgumentText(source, after + chain[0].length - 1).replace(/\s+/g, ' ').trim();
	return `${members}(${argument})`;
}

export function clientAccessesIn(source: string): string[] {
	return namesOfTheClient(source)
		.flatMap((name) => referencesTo(source, name))
		.sort((first, second) => first - second)
		.map((after) => accessAt(source, after))
		.filter((access): access is string => access !== null);
}

export function browserSupabaseCallSites(): CallSiteCounts {
	const callSites: CallSiteCounts = {};
	for (const absolutePath of everySourceFile(sourceRoot)) {
		const path = relative(webRoot, absolutePath);
		if (!isBrowserReachable(path)) continue;
		for (const call of clientAccessesIn(readFileSync(absolutePath, 'utf8'))) {
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
