import { describe, expect, test } from 'bun:test';
import { personName } from '$lib/person-name';

describe('personName', () => {
	test('writes a Korean name family first for a Korean reader', () => {
		expect(personName('표본 김', 'ko')).toBe('김표본');
		expect(personName('견양 박', 'ko')).toBe('박견양');
	});

	test('leaves the recorded order for an English reader', () => {
		expect(personName('표본 김', 'en')).toBe('표본 김');
		expect(personName('John Michael Smith', 'en')).toBe('John Michael Smith');
	});

	test('does not rewrite a Latin name for a Korean reader', () => {
		expect(personName('John Michael Smith', 'ko')).toBe('John Michael Smith');
	});

	test('carries a middle name into the Korean order', () => {
		expect(personName('예시 가운데 박', 'ko')).toBe('박예시가운데');
	});

	test('leaves a name of one part alone', () => {
		expect(personName('김인턴', 'ko')).toBe('김인턴');
		expect(personName('admin', 'ko')).toBe('admin');
		expect(personName('  ', 'ko')).toBe('');
	});
});
