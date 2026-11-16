import { describe, expect, test } from 'bun:test';
import { soulDocumentOf, soulOf } from '../../../src/routes/admin/soul-document';
import { nicknamesWithDefaults } from '../../../src/lib/persona/agent-names';

describe('soulDocumentOf', () => {
	test('writes back the document it was read from', () => {
		const document = { schemaVersion: 1, values: ['확인한 사실만 말합니다.'], boundaries: ['샘플 금지 사항입니다.'], tone: { register: 'formal', traits: ['calm'] }, language: { default: 'ko', matchRequester: true } };
		expect(soulDocumentOf(soulOf(document))).toEqual(document);
	});

	test('drops blank and repeated lines and empty sections', () => {
		const soul = { ...soulOf({}), values: ['  같은 줄 ', '같은 줄', ''], workingStyle: [' '] };
		expect(soulDocumentOf(soul)).toEqual({ schemaVersion: 1, values: ['같은 줄'] });
	});
});

describe('nicknamesWithDefaults', () => {
	test('keeps the default nicknames first and drops blank or repeated additions', () => {
		expect(nicknamesWithDefaults(['샘플봇', '인턴킴', ' ', '샘플봇'])).toEqual(['Intern Kim', '인턴킴', '샘플봇']);
	});
});
