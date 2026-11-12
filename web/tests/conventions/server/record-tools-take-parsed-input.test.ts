import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const recordDirectory = new URL('../../../src/lib/server/public-api/record', import.meta.url).pathname;
const registryPath = join(recordDirectory, 'index.ts');
const untypedInput = /:\s*(Record<string,\s*unknown>|unknown|ToolInput)\s*$/;

function registrations(registry: string): string[] {
	const opened = registry.indexOf('const toolsOverTheRecord');
	const closed = registry.indexOf('\n};', opened);
	return registry.slice(opened, closed).split(/\n\t(?=\w+: recordTool\()/).slice(1);
}

function functionsCalledIn(registration: string): string[] {
	return [...registration.matchAll(/\b([a-z]\w*)\(/g)].map((called) => called[1]);
}

function topLevelParts(list: string): string[] {
	const parts: string[] = [];
	let depth = 0;
	let current = '';
	for (const character of list) {
		if ('<({['.includes(character)) depth += 1;
		if ('>)}]'.includes(character)) depth -= 1;
		if (character === ',' && depth === 0) {
			parts.push(current.trim());
			current = '';
			continue;
		}
		current += character;
	}
	return [...parts, current.trim()].filter(Boolean);
}

function parametersOf(source: string, name: string): string[] | null {
	const declared = new RegExp(`function ${name}\\s*(?:<[^>]*>)?\\(([^)]*\\)?[^)]*)\\)\\s*(?::|\\{)`).exec(source);
	if (!declared) return null;
	return topLevelParts(declared[1]);
}

function handlersTakingUntypedInput(registry: string, modules: string[]): string[] {
	const called = new Set(registrations(registry).flatMap(functionsCalledIn));
	return [...called].flatMap((name) =>
		modules.flatMap((source) => (parametersOf(source, name) ?? []).filter((parameter) => untypedInput.test(parameter)).map((parameter) => `${name}(${parameter})`))
	);
}

function recordModules(): string[] {
	return readdirSync(recordDirectory)
		.filter((entry) => entry.endsWith('.ts'))
		.map((entry) => readFileSync(join(recordDirectory, entry), 'utf8'));
}

describe('a tool over the record', () => {
	test('is handed the input its schema parsed, never a bag of fields it reads again', () => {
		expect(handlersTakingUntypedInput(readFileSync(registryPath, 'utf8'), recordModules())).toEqual([]);
	});

	test('is caught when its handler takes a record of unknown fields', () => {
		const registry = "const toolsOverTheRecord: Record<string, RecordTool> = {\n\tthing_add: recordTool(thingAddInputSchema, (context, input) => thingAdd(context, input))\n};";
		const handler = 'export async function thingAdd(context: RecordContext, input: Record<string, unknown>) {}';
		expect(handlersTakingUntypedInput(registry, [handler])).toEqual(['thingAdd(input: Record<string, unknown>)']);
	});
});
