import { describe, expect, test } from 'bun:test';
import userSchema from '../../../../internal/admind/persona-schema/user.schema.json';
import { speechRegisterName, speechRegisterOf, speechRegisters } from '../../../src/lib/persona/registers';

describe('speech registers', () => {
	test('every register a person can choose has a name and an example in both languages', () => {
		expect(speechRegisters.map((register) => register.value)).toEqual(userSchema.properties.tone.properties.register.enum);
		for (const register of speechRegisters) {
			expect(register.korean.example).not.toBe('');
			expect(register.english.example).not.toBe('');
		}
	});

	test('a register reads as its Korean speech level in Korean and its English name in English', () => {
		expect(speechRegisterName('casual', 'en')).toBe('Casual');
		expect(speechRegisterName('formal', 'ko')).toBe('합쇼체');
		expect(speechRegisterName('plain', 'ko')).toBe('해라체');
		expect(speechRegisterOf('plain')?.english.example).toBe('Confirmed. Will update tomorrow.');
	});

	test('a register the table does not know keeps its stored value', () => {
		expect(speechRegisterName('shouty', 'en')).toBe('shouty');
	});
});
