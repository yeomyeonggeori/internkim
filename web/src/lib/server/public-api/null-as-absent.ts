import { z } from 'zod';

type Wrapped = z.ZodOptional | z.ZodNullable | z.ZodDefault | z.ZodReadonly | z.ZodNonOptional | z.ZodCatch;

function isWrapped(schema: z.core.$ZodType): schema is Wrapped {
	return (
		schema instanceof z.ZodOptional ||
		schema instanceof z.ZodNullable ||
		schema instanceof z.ZodDefault ||
		schema instanceof z.ZodReadonly ||
		schema instanceof z.ZodNonOptional ||
		schema instanceof z.ZodCatch
	);
}

function unwrapped(schema: z.core.$ZodType): z.core.$ZodType {
	if (isWrapped(schema)) return unwrapped(schema.unwrap());
	if (schema instanceof z.ZodPipe) return unwrapped(schema.in);
	return schema;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function spelledAsNothing(field: z.core.$ZodType): null | undefined {
	if (z.safeParse(field, undefined).success) return undefined;
	return z.safeParse(field, null).success ? null : undefined;
}

function fieldsReadUnder(shape: Record<string, z.core.$ZodType>, value: Record<string, unknown>): Record<string, unknown> {
	const read: Record<string, unknown> = {};
	for (const [name, field] of Object.entries(value)) {
		if (field !== null) read[name] = name in shape ? readWithNullAsAbsent(shape[name], field) : field;
	}
	for (const [name, field] of Object.entries(shape)) {
		if (read[name] === undefined && spelledAsNothing(field) === null) read[name] = null;
	}
	return read;
}

export function readWithNullAsAbsent(schema: z.core.$ZodType, value: unknown): unknown {
	const inner = unwrapped(schema);
	if (inner instanceof z.ZodObject && isRecord(value)) return fieldsReadUnder(inner.shape, value);
	if (inner instanceof z.ZodArray && Array.isArray(value)) {
		return value.map((item) => (item === null ? item : readWithNullAsAbsent(inner.element, item)));
	}
	return value;
}
