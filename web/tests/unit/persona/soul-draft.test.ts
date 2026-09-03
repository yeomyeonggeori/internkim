import { describe, expect, test } from 'bun:test';
import { draftToSoul, draftToUser, soulToDraft, userToDraft } from '../../../src/lib/persona/soul-draft';

describe('soul draft', () => {
	test('a soul round-trips through the form draft', () => {
		const soul = {
			schemaVersion: 1 as const,
			values: ['Lead with the result.'],
			boundaries: ['Never read another person\'s direct messages.'],
			tone: { register: 'polite' as const, traits: ['warm'] },
			language: { default: 'ko', matchRequester: true }
		};
		expect(draftToSoul(soulToDraft(soul))).toEqual(soul);
	});

	test('blank lines and an unset tone leave no field behind', () => {
		const soul = draftToSoul({
			valuesText: '\n  \n',
			boundariesText: '',
			workingStyleText: 'Ask once, then act.\n\n',
			register: '',
			traitsText: '',
			languageDefault: ' ',
			matchRequester: false
		});
		expect(soul).toEqual({ schemaVersion: 1, workingStyle: ['Ask once, then act.'] });
	});
});

describe('user draft', () => {
	test('a user document round-trips and carries no matchRequester', () => {
		const user = {
			schemaVersion: 1 as const,
			callMe: '샘플님',
			preferences: ['Give me the command first.'],
			tone: { register: 'casual' as const },
			language: { default: 'en' }
		};
		expect(draftToUser(userToDraft(user))).toEqual(user);
		expect(Object.keys(draftToUser(userToDraft({ schemaVersion: 1 })))).toEqual(['schemaVersion']);
	});
});
