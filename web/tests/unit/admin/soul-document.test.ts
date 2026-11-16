import { describe, expect, test } from 'bun:test';
import { soulChangesBetween, soulOf } from '../../../src/routes/admin/soul-document';

describe('working principle changes', () => {
	test('a rewritten line reads as the old line removed and the new one added in its column', () => {
		const before = soulOf({ schemaVersion: 1, values: ['Lead with the result.', 'Check before saying done.'] });
		const after = soulOf({ schemaVersion: 1, values: ['Put the conclusion first.', 'Check before saying done.'] });
		expect(soulChangesBetween(before, after)).toEqual([
			{ field: 'values', removed: 'Lead with the result.' },
			{ field: 'values', added: 'Put the conclusion first.' }
		]);
	});

	test('a tone or language change is named once with both sides', () => {
		const before = soulOf({ schemaVersion: 1, tone: { register: 'formal', traits: ['calm'] }, language: { matchRequester: true } });
		const after = soulOf({ schemaVersion: 1, tone: { register: 'plain', traits: ['calm', 'clear'] }, language: { default: 'ko', matchRequester: true } });
		const changes = soulChangesBetween(before, after);
		expect(changes.map((change) => change.field)).toEqual(['register', 'traits', 'language']);
		expect(changes[0]).toEqual({ field: 'register', removed: 'formal', added: 'plain' });
	});

	test('identical documents have no changes, and a malformed one reads as empty', () => {
		const document = soulOf({ schemaVersion: 1, boundaries: ['Never share a secret.'] });
		expect(soulChangesBetween(document, document)).toEqual([]);
		expect(soulOf('not a document')).toEqual({ values: [], boundaries: [], workingStyle: [], register: '', traits: [], languageDefault: '', matchRequester: false });
	});
});
