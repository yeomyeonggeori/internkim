import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { z } from 'zod';
import { companyModelPayers, payerOfTheModels } from '$lib/company/credits-contract';

const whoPaysCases = z.array(
	z.strictObject({ holdsKey: z.boolean(), usesOwnKey: z.boolean(), creditsCanPay: z.boolean(), paidBy: z.enum(companyModelPayers) })
);

const cases = whoPaysCases.parse(
	JSON.parse(readFileSync(new URL('../../../../internal/modelladder/testdata/who-pays.json', import.meta.url), 'utf8'))
);

describe('who pays for the models', () => {
	test('the screen answers every case the way the company computer decides it', () => {
		for (const { paidBy, ...facts } of cases) expect({ ...facts, paidBy: payerOfTheModels(facts) }).toEqual({ ...facts, paidBy });
	});

	test('the cases cover every combination of the three facts', () => {
		const combinations = new Set(cases.map((rule) => `${rule.holdsKey}${rule.usesOwnKey}${rule.creditsCanPay}`));
		expect(combinations.size).toBe(8);
	});
});
