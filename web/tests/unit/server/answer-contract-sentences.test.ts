import { describe, expect, test } from 'bun:test';
import { capabilityToolResultSchema } from '../../../src/lib/server/public-api/catalog/tools';
import { sentencesOfSchemaRefusal } from '../../../src/lib/server/public-api/schema-sentences';

function refusalOf(tool: string, result: unknown): string {
	const schema = capabilityToolResultSchema(tool);
	if (!schema) throw new Error(`${tool} publishes no result contract`);
	const parsed = schema.safeParse(result);
	if (parsed.success) throw new Error(`${tool} accepted an answer this test expects it to refuse`);
	return sentencesOfSchemaRefusal(parsed.error.issues, result, 'result');
}

// A device reads a record tool's answer through a copy of this contract that
// only an OTA release refreshes, so it carries the answer and this side is
// where a break is named. The sentences are the ones the capability runtime
// writes in Go for the same faults, so the reader sees one vocabulary
// whichever side names it and whichever direction the contract runs.
describe('an answer that left its contract is named the way the input gate names a call', () => {
	test('a promised list answered as null names the field and both types', () => {
		expect(refusalOf('task_list', { tasks: null, count: 0, scope: 'self', registeredLabels: {} })).toContain(
			'result.tasks must be an array, and it is null'
		);
	});

	test('a promised field left out is named as missing', () => {
		expect(refusalOf('task_add', { status: 'planned' })).toContain('result.taskID is required and is missing');
	});

	test('the member of a list that broke is named by its index', () => {
		const refusal = refusalOf('task_list', {
			tasks: [{ taskID: 'task-1' }, { taskID: 7 }],
			count: 2,
			scope: 'self',
			registeredLabels: { businesses: [], types: [], sizes: [], statuses: [] }
		});
		expect(refusal).toContain('result.tasks.1.taskID must be a string, and it is 7');
	});

	test('a refusal never speaks in schema pointers', () => {
		expect(refusalOf('task_add', { status: 'planned' })).not.toContain('/properties/');
	});
});
