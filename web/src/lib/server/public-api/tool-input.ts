import type { $ZodIssue } from 'zod/v4/core';
import { capabilityToolInputSchema } from './catalog/tools';

type FieldSchema = { safeParse(value: unknown): { success: boolean } };

// The caller this API was built for is a model, and a model has three ways of
// saying a field has no value: it leaves the field out, it writes null, or it
// writes "". Only the first was taken. null is not undefined to a schema, and
// "" is not one of XS|S|M to an enum, so two of the three were refused for
// their spelling — 52 of 105 checks over the catalog.
//
// Widening the schemas is not the fix: a nullable type is the shape the
// tool-calling backends disagree about, and the catalog is written to the least
// common denominator on purpose. The three are made one here instead. A field
// the schema lets be left out, written with nothing in it, is left out — which
// is what the caller said, in the spelling the schema already understands.
//
// A field that must be given is untouched: dropping a blank there would turn
// "you sent nothing" into "you forgot this", and the caller needs the first.
function fieldsTheToolLetsBeLeftOut(name: string): Map<string, FieldSchema> {
	const schema = capabilityToolInputSchema(name);
	const shape = (schema as unknown as { shape?: Record<string, FieldSchema> })?.shape;
	if (!shape) return new Map();
	return new Map(Object.entries(shape).filter(([, field]) => field.safeParse(undefined).success));
}

// null is not a value any of these schemas take, so it is always the caller
// saying nothing. An empty string is one only where the field refuses it: "" is
// not one of XS|S|M, so it means the size was left out. A field that takes an
// empty string takes it as a value, and the catalog says what it means on the
// fields that do — an empty organizationHint takes the work off the
// organization rather than leaving it where it was.
function saysNothing(value: unknown, field: FieldSchema): boolean {
	if (value === null) return true;
	if (typeof value !== 'string' || value.trim() !== '') return false;
	return !field.safeParse(value).success;
}

export function toolInputRecovered(name: string, input: unknown): Record<string, unknown> {
	if (input === null || input === undefined) return {};
	if (typeof input !== 'object' || Array.isArray(input)) return input as Record<string, unknown>;
	const mayBeLeftOut = fieldsTheToolLetsBeLeftOut(name);
	return Object.fromEntries(
		Object.entries(input as Record<string, unknown>).filter(([field, value]) => {
			const schema = mayBeLeftOut.get(field);
			return !schema || !saysNothing(value, schema);
		})
	);
}

export function refusalOfToolInput(name: string, input: unknown): string | null {
	const schema = capabilityToolInputSchema(name);
	if (!schema) return null;
	const parsed = schema.safeParse(toolInputRecovered(name, input));
	if (parsed.success) return null;
	return parsed.error.issues.map(sentenceOfIssue).join('; ');
}

function sentenceOfIssue(issue: $ZodIssue): string {
	return `${fieldsNamedBy(issue).join(', ')}: ${issue.message}`;
}

function fieldsNamedBy(issue: $ZodIssue): string[] {
	if (issue.code === 'unrecognized_keys') {
		return issue.keys.map((key) => fieldName([...issue.path, key]));
	}
	return [fieldName(issue.path)];
}

function fieldName(path: ReadonlyArray<PropertyKey>): string {
	return ['input', ...path.map(String)].join('.');
}
