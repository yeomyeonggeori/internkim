import { describe, expect, test } from 'bun:test';
import {
	anIssuedPassword,
	isAlreadyClaimed,
	mailCanCarryTheCode
} from '../../../src/lib/server/claim-without-mail';

describe('mailCanCarryTheCode', () => {
	test('a deployment that says nothing keeps the emailed code', () => {
		expect(mailCanCarryTheCode({})).toBe(true);
	});

	test('only the exact opt-in turns the emailed code off', () => {
		expect(mailCanCarryTheCode({ AUTH_CLAIM_WITHOUT_EMAIL: '1' })).toBe(false);
		expect(mailCanCarryTheCode({ AUTH_CLAIM_WITHOUT_EMAIL: ' 1 ' })).toBe(false);
	});

	test('anything else leaves it on, so a stray value cannot open the door', () => {
		for (const value of ['0', '', 'true', 'yes', 'no', 'off']) {
			expect(mailCanCarryTheCode({ AUTH_CLAIM_WITHOUT_EMAIL: value })).toBe(true);
		}
	});
});

describe('isAlreadyClaimed', () => {
	test('an account that carries a sign-in identity has been claimed', () => {
		expect(isAlreadyClaimed({ identities: [{ provider: 'email' }] })).toBe(true);
	});

	test('an account nobody has claimed carries none', () => {
		expect(isAlreadyClaimed({ identities: [] })).toBe(false);
		expect(isAlreadyClaimed({ identities: null })).toBe(false);
		expect(isAlreadyClaimed({})).toBe(false);
	});

	test('no account at all is not a claimed one', () => {
		expect(isAlreadyClaimed(undefined)).toBe(false);
	});
});

describe('anIssuedPassword', () => {
	test('it is long enough to be worth issuing', () => {
		expect(anIssuedPassword().length).toBe(20);
	});

	test('it carries no company, person, or year to guess from', () => {
		const drawn = Array.from({ length: 200 }, () => anIssuedPassword());
		expect(new Set(drawn).size).toBe(drawn.length);
		for (const password of drawn) {
			expect(password).not.toContain('2026');
			expect(/^[A-HJ-NP-Za-km-z2-9]+$/.test(password)).toBe(true);
		}
	});

	test('every position varies, so no prefix is fixed', () => {
		const drawn = Array.from({ length: 200 }, () => anIssuedPassword());
		for (let position = 0; position < 20; position += 1) {
			expect(new Set(drawn.map((password) => password[position])).size > 1).toBe(true);
		}
	});
});
