import { describe, expect, test } from 'bun:test';
import { positiveNumberSetting } from './settings';

describe('positiveNumberSetting', () => {
	test('an unset variable takes the fallback', () => {
		expect(positiveNumberSetting('ANSWER_BYTE_CEILING', undefined, 900_000)).toBe(900_000);
	});

	test('a variable left empty takes the fallback, never zero', () => {
		expect(positiveNumberSetting('ANSWER_BYTE_CEILING', '', 900_000)).toBe(900_000);
		expect(positiveNumberSetting('ANSWER_BYTE_CEILING', '  ', 900_000)).toBe(900_000);
	});

	test('a number is read, with the whitespace an env file leaves', () => {
		expect(positiveNumberSetting('ANSWER_BYTE_CEILING', ' 250000 ', 900_000)).toBe(250_000);
	});

	test('a value that is not a number stops the boot instead of becoming NaN', () => {
		expect(() => positiveNumberSetting('ANSWER_BYTE_CEILING', '1MB', 900_000)).toThrow(
			'ANSWER_BYTE_CEILING must be a positive number, not "1MB"'
		);
	});

	test('zero and below stop the boot, because they refuse every answer', () => {
		expect(() => positiveNumberSetting('ANSWER_BYTE_CEILING', '0', 900_000)).toThrow();
		expect(() => positiveNumberSetting('ANSWER_BYTE_CEILING', '-1', 900_000)).toThrow();
	});
});
