import { describe, expect, test } from 'bun:test';
import { capabilityToolInputSchema } from '../../../src/lib/server/public-api/catalog/tools';
import { refusalOfToolInput, toolInputRecovered } from '../../../src/lib/server/public-api/tool-input';
import catalog from '../../../../pkg/capabilityprotocol/generated/capability-tools.json';

type FieldSchema = { safeParse(value: unknown): { success: boolean } };

const toolNames = catalog.tools.map((tool) => tool.name);
const waysOfSayingNothing: [string, unknown][] = [
	['null', null],
	['a blank', ''],
	['spaces', '   ']
];

function fieldsTheToolLetsBeLeftOut(name: string): string[] {
	const schema = capabilityToolInputSchema(name);
	if (!schema) throw new Error(`${name} publishes no input contract`);
	const shape = (schema as unknown as { shape?: Record<string, FieldSchema> }).shape;
	if (!shape) throw new Error(`${name} does not publish its fields`);
	return Object.keys(shape).filter((field) => shape[field].safeParse(undefined).success);
}

// A tool with required fields refuses an empty call for missing them, and that
// says nothing about the field under test. What matters is what writing the
// field adds, so the complaints an empty call already makes are the baseline.
function complaintsAdded(name: string, field: string, value: unknown): string[] {
	const already = new Set((refusalOfToolInput(name, {}) ?? '').split('; ').filter(Boolean));
	const now = (refusalOfToolInput(name, { [field]: value }) ?? '').split('; ').filter(Boolean);
	return now.filter((complaint) => !already.has(complaint));
}

// The caller this API was built for is a model, and a model has three ways of
// saying a field has no value: it leaves the field out, it writes null, or it
// writes "". They are one request in three spellings, and a contract that takes
// one and refuses the others turns an ordinary call into a refusal about
// nothing. "‘하드웨어 기획서’를 예정 업무로 등록하지 못했습니다" was that refusal.
describe('every way of saying a field has no value is the same way', () => {
	for (const name of toolNames) {
		const optional = fieldsTheToolLetsBeLeftOut(name);
		if (optional.length === 0) continue;

		for (const [spelling, value] of waysOfSayingNothing) {
			test(`${name} reads ${spelling} as the field being left out`, () => {
				const refused = optional.filter((field) => complaintsAdded(name, field, value).length > 0);
				expect({ tool: name, refused }).toEqual({ tool: name, refused: [] });
			});
		}
	}
});

describe('a whole call written the way a model writes one', () => {
	for (const [spelling, value] of waysOfSayingNothing) {
		test(`task_add takes ${spelling} through every field it lets be left out`, () => {
			const written = Object.fromEntries(
				fieldsTheToolLetsBeLeftOut('task_add').map((field) => [field, value])
			);
			expect(refusalOfToolInput('task_add', { ...written, title: '모델이 쓴 그대로' })).toBeNull();
		});
	}

	test('a field the tool does not take is still named back', () => {
		expect(refusalOfToolInput('task_add', { title: '있는 업무', mood: 'cheerful' })).toContain('input.mood');
	});

	// Recovering a blank here would turn "you sent nothing" into "you forgot
	// this", and the caller needs to be told the first.
	test('a field that must be given is refused by name when it is sent empty', () => {
		expect(refusalOfToolInput('task_add', { title: null })).toContain('input.title');
		expect(refusalOfToolInput('task_add', { title: '' })).toBeNull();
	});
});

describe('a blank the field itself takes', () => {
	test('is kept, because it is what takes a task out from under its parent', () => {
		expect(toolInputRecovered('task_update', { taskHint: 'a-task', parentTaskHint: '' })).toEqual({
			taskHint: 'a-task',
			parentTaskHint: ''
		});
		expect(refusalOfToolInput('task_update', { taskHint: 'a-task', parentTaskHint: '' })).toBeNull();
	});

	test('is kept for a date, which is what takes the date off', () => {
		expect(toolInputRecovered('task_update', { taskHint: 'a-task', endsAt: '' })).toEqual({
			taskHint: 'a-task',
			endsAt: ''
		});
	});

	test('is dropped where the field refuses one, so a blank size is a size left out', () => {
		expect(toolInputRecovered('task_update', { taskHint: 'a-task', size: '', status: '' })).toEqual({
			taskHint: 'a-task'
		});
	});

	test('is dropped when it is null, which no field here takes', () => {
		expect(toolInputRecovered('task_update', { taskHint: 'a-task', parentTaskHint: null })).toEqual({
			taskHint: 'a-task'
		});
	});
});

// A device holds the catalog its release shipped with, so the fleet keeps
// asking under a retired name for as long as it takes an OTA to reach it.
// Refusing that call cannot be recovered from: the device's own catalog refuses
// the name that replaced it, so the reply has nothing to ask for.
describe('a call written against the catalog a device still holds', () => {
	test('is read under the name that replaced the retired one', () => {
		expect(toolInputRecovered('task_list', { participantPersonHint: '박예시' })).toEqual({
			personHints: ['박예시']
		});
		expect(toolInputRecovered('attendance_list', { personHint: '박예시' })).toEqual({
			personHints: ['박예시']
		});
	});

	// scope only ever answered whose records to read when the hint was empty.
	test('keeps the meaning the retired catalog gave a call that carried both', () => {
		expect(toolInputRecovered('leave_list', { personHint: '박예시', scope: 'self' })).toEqual({
			personHints: ['박예시']
		});
	});

	test('leaves a scope that came on its own alone', () => {
		expect(toolInputRecovered('leave_list', { scope: 'all' })).toEqual({ scope: 'all' });
	});

	test('leaves the field alone where it is still the name the tool publishes', () => {
		expect(toolInputRecovered('attendance_add', { personHint: '박예시', kind: 'clock_in' })).toEqual({
			personHint: '박예시',
			kind: 'clock_in'
		});
	});
});
