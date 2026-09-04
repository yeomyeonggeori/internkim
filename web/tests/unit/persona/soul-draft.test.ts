import { describe, expect, test } from 'bun:test';
import { draftToSoul, draftToUser, soulToDraft, toneTraitLimit, userToDraft } from '../../../src/lib/persona/soul-draft';

describe('soul draft', () => {
	test('a soul round-trips through the form draft', () => {
		const soul = {
			schemaVersion: 1 as const,
			values: ['Lead with the result.'],
			boundaries: ["Never read another person's direct messages."],
			tone: { register: 'polite' as const, traits: ['warm'] },
			language: { default: 'ko', matchRequester: true }
		};
		expect(draftToSoul(soulToDraft(soul))).toEqual(soul);
	});

	test('an empty document drafts to formal with no traits and keeps only the tone', () => {
		const draft = soulToDraft({ schemaVersion: 1 });
		expect(draft.register).toBe('formal');
		expect(draft.traits).toEqual([]);
		const soul = draftToSoul({
			valuesText: '\n  \n',
			boundariesText: '',
			workingStyleText: 'Ask once, then act.\n\n',
			register: 'formal',
			traits: [],
			languageDefault: ' ',
			matchRequester: false
		});
		expect(soul).toEqual({ schemaVersion: 1, workingStyle: ['Ask once, then act.'], tone: { register: 'formal' } });
	});

	test('legacy korean traits become canonical tokens and unknown traits survive', () => {
		const draft = soulToDraft({
			schemaVersion: 1,
			tone: { register: 'polite', traits: ['차분한', '또렷한', '간결한', '따뜻한', 'stoic'] }
		});
		expect(draft.traits).toEqual(['calm', 'clear', 'concise', 'warm', 'stoic']);
	});

	test('the trait limit caps what a draft emits', () => {
		const soul = draftToSoul({
			valuesText: '',
			boundariesText: '',
			workingStyleText: '',
			register: 'casual',
			traits: ['calm', 'warm', 'clear', 'concise', 'direct', 'playful', 'meticulous'],
			languageDefault: '',
			matchRequester: false
		});
		expect(soul.tone?.traits).toHaveLength(toneTraitLimit);
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
	});

	test('an empty user document still saves a formal tone', () => {
		expect(draftToUser(userToDraft({ schemaVersion: 1 }))).toEqual({
			schemaVersion: 1,
			tone: { register: 'formal' }
		});
	});
});
