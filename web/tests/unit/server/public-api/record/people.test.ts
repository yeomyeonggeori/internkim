import { describe, expect, test } from 'bun:test';
import { candidateOf, mentionOf, personOfHint, peopleOfHints } from '$lib/server/public-api/record/people';
import { HintRefused } from '$lib/server/public-api/record/hint-resolution';
import { personInTheDirectory } from './directory-fixture';

const people = [
	{ personID: 'm1', name: '이샘플', email: 'sample@example.com' },
	{ personID: 'm2', name: '박예시', email: 'yesi@example.com' },
	{ personID: 'm3', name: '박예시연', email: 'yesiyeon@example.com' }
].map(personInTheDirectory);

function refusalOf(hint: string): HintRefused {
	try {
		personOfHint(people, hint);
	} catch (thrown) {
		if (thrown instanceof HintRefused) return thrown;
		throw thrown;
	}
	throw new Error(`${hint} was expected to be refused`);
}

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

	test('takes the @handle a candidate hands back, which is not the mention', () => {
		expect(personOfHint(people, '@yesi').personID).toBe('m2');
		expect(candidateOf(people[1]).handle).toBe('@yesi');
		expect(personOfHint(people, candidateOf(people[1]).handle as string).personID).toBe('m2');
		expect(mentionOf(people[1].name)).toBe('@박예시');
	});

	test('asks rather than picking one when two addresses share a local part', () => {
		const sharing = [...people, personInTheDirectory({ personID: 'm4', name: '박예시', email: 'yesi@example.co.kr' })];
		const refusal = (() => {
			try {
				personOfHint(sharing, '@yesi');
			} catch (thrown) {
				if (thrown instanceof HintRefused) return thrown;
				throw thrown;
			}
			throw new Error('the shared handle was expected to be refused');
		})();
		expect(refusal.outcome).toBe('ambiguous');
		expect(refusal.candidates.map((one) => one.id).sort()).toEqual(['m2', 'm4']);
	});

	test('refuses a part two people answer to, and says who they are', () => {
		const refusal = refusalOf('박');
		expect(refusal.outcome).toBe('ambiguous');
		expect(refusal.errorCode).toBe('interaction_required');
		expect(refusal.candidates.map((one) => one.label)).toEqual(['박예시', '박예시연']);
		expect(refusal.candidates[0]).toEqual({
			id: 'm2',
			label: '박예시',
			email: 'yesi@example.com',
			handle: '@yesi'
		});
	});

	test('offers the nearest when a name is a character off, and resolves nothing', () => {
		const refusal = refusalOf('박예시연연');
		expect(refusal.outcome).toBe('approximate');
		expect(refusal.errorCode).toBe('interaction_required');
		expect(refusal.candidates.map((one) => one.label)).toContain('박예시연');
	});

	test('offers the nearest for an address whose local part is a character off', () => {
		const refusal = refusalOf('sampl@example.com');
		expect(refusal.outcome).toBe('approximate');
		expect(refusal.candidates.map((one) => one.label)).toContain('이샘플');
	});

	test('offers the nearest for a mistyped domain, and nothing for a local part nobody has', () => {
		const domainSlip = refusalOf('yesi@example.con');
		expect(domainSlip.outcome).toBe('approximate');
		expect(domainSlip.candidates.map((one) => one.email)).toContain('yesi@example.com');

		expect(refusalOf('nobodyhere@example.com').outcome).toBe('not_found');
	});

	test('refuses a name nothing comes close to, with no candidates', () => {
		const refusal = refusalOf('최견본');
		expect(refusal.outcome).toBe('not_found');
		expect(refusal.errorCode).toBe('person_not_found');
		expect(refusal.candidates).toEqual([]);
		expect(refusalOf('  ').outcome).toBe('not_found');
	});

	test('names the role the hint was given in', () => {
		expect(() => personOfHint(people, '최견본', 'participant')).toThrow(HintRefused);
		const refusal = (() => {
			try {
				personOfHint(people, '최견본', 'participant');
			} catch (thrown) {
				return thrown as HintRefused;
			}
			throw new Error('the hint was expected to be refused');
		})();
		expect(refusal.errorCode).toBe('task_participant_not_found');
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
		expect(() => peopleOfHints(people, ['m1', '없는사람'])).toThrow(HintRefused);
	});
});
