import { describe, expect, test } from 'bun:test';
import { capabilityToolInputSchema } from '../../../src/lib/server/public-api/catalog/tools';
import { refusalOfToolInput } from '../../../src/lib/server/public-api/tool-input';
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
