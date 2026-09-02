import type { $ZodIssue } from 'zod/v4/core';
import { capabilityToolInputSchema } from './catalog/tools';

export function refusalOfToolInput(name: string, input: unknown): string | null {
	const schema = capabilityToolInputSchema(name);
	if (!schema) return null;
	const parsed = schema.safeParse(input ?? {});
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
