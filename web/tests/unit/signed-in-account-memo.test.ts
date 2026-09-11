import { beforeEach, describe, expect, test } from 'bun:test';
import { companyMembership, forgetSignedInAccount } from '$lib/signed-in-account-memo';

function countedLookup(answers: Array<boolean | Error>) {
	let calls = 0;
	return {
		lookUp: async (): Promise<boolean> => {
			const answer = answers[Math.min(calls, answers.length - 1)];
			calls += 1;
			if (answer instanceof Error) throw answer;
			return answer;
		},
		calls: () => calls
	};
}

describe('signed-in account memo', () => {
	beforeEach(forgetSignedInAccount);

	test('asks once for an account that belongs to a company', async () => {
		const lookup = countedLookup([true]);
		await companyMembership.of('account-a', lookup.lookUp);
		expect(await companyMembership.of('account-a', lookup.lookUp)).toBe(true);
		expect(lookup.calls()).toBe(1);
	});

	test('shares a lookup that is still in flight', async () => {
		const lookup = countedLookup([true]);
		await Promise.all([
			companyMembership.of('account-a', lookup.lookUp),
			companyMembership.of('account-a', lookup.lookUp)
		]);
		expect(lookup.calls()).toBe(1);
	});

	test('asks again for a different account', async () => {
		const lookup = countedLookup([true]);
		await companyMembership.of('account-a', lookup.lookUp);
		await companyMembership.of('account-b', lookup.lookUp);
		expect(lookup.calls()).toBe(2);
	});

	test('asks again when the answer was not worth keeping', async () => {
		const lookup = countedLookup([false, true]);
		expect(await companyMembership.of('account-a', lookup.lookUp)).toBe(false);
		expect(await companyMembership.of('account-a', lookup.lookUp)).toBe(true);
		expect(lookup.calls()).toBe(2);
	});

	test('asks again after a failed lookup', async () => {
		const lookup = countedLookup([new Error('member lookup returned 503'), true]);
		expect(companyMembership.of('account-a', lookup.lookUp)).rejects.toThrow('member lookup returned 503');
		await Promise.resolve();
		expect(await companyMembership.of('account-a', lookup.lookUp)).toBe(true);
		expect(lookup.calls()).toBe(2);
	});

	test('asks again once the signed-in account is forgotten', async () => {
		const lookup = countedLookup([true]);
		await companyMembership.of('account-a', lookup.lookUp);
		forgetSignedInAccount();
		await companyMembership.of('account-a', lookup.lookUp);
		expect(lookup.calls()).toBe(2);
	});
});
