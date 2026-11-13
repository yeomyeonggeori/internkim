import { describe, expect, test } from 'bun:test';
import { personOfHint } from '$lib/server/public-api/record/people';
import { HintRefused } from '$lib/server/public-api/record/hint-resolution';
import sharedCases from '$lib/person-reference-cases.json';
import { personInTheDirectory } from './directory-fixture';

type SharedCompany = keyof typeof sharedCases.companies;

function isSharedCompany(company: string): company is SharedCompany {
	return company in sharedCases.companies;
}

function peopleOf(company: string) {
	if (!isSharedCompany(company)) throw new Error(`the shared cases name no company ${company}`);
	return sharedCases.companies[company].map(personInTheDirectory);
}

function answerTo(company: string, hint: string): { outcome: string; personIDs: string[] } {
	try {
		return { outcome: 'resolved', personIDs: [personOfHint(peopleOf(company), hint).personID] };
	} catch (thrown) {
		if (!(thrown instanceof HintRefused)) throw thrown;
		return { outcome: thrown.outcome, personIDs: thrown.candidates.map((candidate) => candidate.id) };
	}
}

function expectedPersonIDs(sharedCase: { match?: string; candidates?: string[] }): string[] {
	if (sharedCase.match !== undefined) return [sharedCase.match];
	return sharedCase.candidates ?? [];
}

describe('a reference to a person names whom the shared cases say', () => {
	test('there are shared cases to read', () => {
		expect(sharedCases.cases.length > 0).toBe(true);
	});

	for (const sharedCase of sharedCases.cases) {
		test(`${sharedCase.hint} in ${sharedCase.company}`, () => {
			expect(answerTo(sharedCase.company, sharedCase.hint)).toEqual({
				outcome: sharedCase.outcome,
				personIDs: expectedPersonIDs(sharedCase)
			});
		});
	}
});
