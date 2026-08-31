import { describe, expect, test } from 'bun:test';
import { HintUnresolved, personOfHint, peopleOfHints } from '$lib/server/public-api/record/people';

const people = [
	{ personID: 'm1', name: '이샘플', email: 'sample@example.com' },
	{ personID: 'm2', name: '박예시', email: 'yesi@example.com' },
	{ personID: 'm3', name: '박예시연', email: 'yesiyeon@example.com' }
];

describe('naming a person', () => {
	test('takes an id or an address exactly, whatever the case', () => {
		expect(personOfHint(people, 'm2').personID).toBe('m2');
		expect(personOfHint(people, 'Sample@Example.com').personID).toBe('m1');
	});

	test('takes a whole name even when another name contains it', () => {
		expect(personOfHint(people, '박예시').personID).toBe('m2');
	});

	test('takes a part of a name when exactly one person answers to it', () => {
		expect(personOfHint(people, '샘플').personID).toBe('m1');
		expect(personOfHint(people, '예시연').personID).toBe('m3');
	});

	test('refuses a part two people answer to, and says who they are', () => {
		const refusal = (() => {
			try {
				personOfHint(people, '박');
			} catch (thrown) {
				return thrown as HintUnresolved;
			}
			throw new Error('the hint was expected to be refused');
		})();
		expect(refusal).toBeInstanceOf(HintUnresolved);
		expect(refusal.candidates).toEqual(['박예시', '박예시연']);
	});

	test('refuses a name nobody answers to', () => {
		expect(() => personOfHint(people, '최견본')).toThrow(HintUnresolved);
		expect(() => personOfHint(people, '  ')).toThrow(HintUnresolved);
	});
});

describe('naming several people', () => {
	test('keeps the order asked for and names nobody twice', () => {
		expect(peopleOfHints(people, ['박예시', 'm1', 'sample@example.com']).map((one) => one.personID)).toEqual([
			'm2',
			'm1'
		]);
	});

	test('refuses the whole list when one name is refused', () => {
		expect(() => peopleOfHints(people, ['m1', '없는사람'])).toThrow(HintUnresolved);
	});
});
