import type { $ZodIssue } from 'zod/v4/core';

// A model reads a refusal to decide what to send next, and zod's own wording
// says what the field is not without saying what it holds: "Invalid option:
// expected one of "self"|"all"" names neither the field nor the value that was
// refused. These are the sentences the capability runtime writes in Go for the
// same faults, so a caller reads one vocabulary whichever side refuses it and
// whichever direction the contract runs.
export function sentencesOfSchemaRefusal(
	issues: ReadonlyArray<$ZodIssue>,
	given: unknown,
	subject: string
): string {
	return issues.map((issue) => sentenceOfIssue(issue, given, subject)).join('; ');
}

function sentenceOfIssue(issue: $ZodIssue, given: unknown, subject: string): string {
	if (issue.code === 'unrecognized_keys') {
		return issue.keys
			.map((key) => `${fieldName([...issue.path, key], subject)} is not a field this tool takes`)
			.join('; ');
	}
	const field = fieldName(issue.path, subject);
	const value = valueAt(given, issue.path);
	if (value === undefined) return `${field} is required and is missing`;
	if (issue.code === 'invalid_value') {
		return `${field} must be one of ${issue.values.map(renderValue).join(', ')}, and it is ${renderValue(value)}`;
	}
	if (issue.code === 'invalid_type') {
		return `${field} must be ${article(issue.expected)}, and it is ${renderValue(value)}`;
	}
	return `${field}: ${issue.message}`;
}

function valueAt(given: unknown, path: ReadonlyArray<PropertyKey>): unknown {
	return path.reduce<unknown>((reached, step) => {
		if (reached === null || typeof reached !== 'object') return undefined;
		return (reached as Record<PropertyKey, unknown>)[step];
	}, given);
}

function article(expected: string): string {
	if (expected === 'array' || expected === 'object' || expected === 'integer') return `an ${expected}`;
	if (expected === 'null') return 'null';
	return `a ${expected}`;
}

const renderedValueLimit = 80;

function renderValue(value: unknown): string {
	const rendered = JSON.stringify(value) ?? String(value);
	return rendered.length <= renderedValueLimit ? rendered : `${rendered.slice(0, renderedValueLimit)}…`;
}

function fieldName(path: ReadonlyArray<PropertyKey>, subject: string): string {
	return [subject, ...path.map(String)].join('.');
}
