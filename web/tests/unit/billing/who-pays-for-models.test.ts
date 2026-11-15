import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { z } from 'zod';
import { isAModelKeyHeld, payerOfTheModels, shownModelPayers } from '$lib/company/credits-contract';

const whoPaysCases = z.array(
	z.strictObject({ holdsKey: z.boolean().nullable(), usesOwnKey: z.boolean(), creditsCanPay: z.boolean(), paidBy: z.enum(shownModelPayers) })
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
		expect(combinations.size).toBe(12);
	});

	test('a key the plane keeps is held, and otherwise the computer is believed, or nobody knows yet', () => {
		expect(isAModelKeyHeld(true, null)).toBe(true);
		expect(isAModelKeyHeld(true, false)).toBe(true);
		expect(isAModelKeyHeld(false, true)).toBe(true);
		expect(isAModelKeyHeld(false, false)).toBe(false);
		expect(isAModelKeyHeld(false, null)).toBeNull();
	});
});
