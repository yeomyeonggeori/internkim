import { describe, expect, test } from 'bun:test';
import { draftToUser, toneTraitLimit, userToDraft } from '../../../src/lib/persona/soul-draft';

describe('user draft', () => {
	test('a user document round-trips and carries no matchRequester', () => {
		const user = {
			schemaVersion: 1 as const,
			callMe: '샘플님',
			preferences: ['Give me the command first.'],
			tone: { register: 'casual' as const },
			language: { default: 'en' },
			morningBriefing: { enabled: false, time: '09:30' }
		};
		expect(draftToUser(userToDraft(user))).toEqual(user);
	});

	test('a tone change preserves the morning briefing settings', () => {
		const draft = userToDraft({ schemaVersion: 1, morningBriefing: { enabled: false, time: '09:30' } });
		draft.register = 'casual';
		expect(draftToUser(draft).morningBriefing).toEqual({ enabled: false, time: '09:30' });
	});

	test('legacy korean traits become canonical tokens and unknown traits survive', () => {
		const draft = userToDraft({
			schemaVersion: 1,
			tone: { register: 'polite', traits: ['차분한', '또렷한', '간결한', '따뜻한', 'stoic'] }
		});
		expect(draft.traits).toEqual(['calm', 'clear', 'concise', 'warm', 'stoic']);
	});

	test('the trait limit caps what a draft emits', () => {
		const draft = userToDraft({ schemaVersion: 1 });
		draft.traits = ['calm', 'warm', 'clear', 'concise', 'direct', 'playful', 'meticulous'];
		expect(draftToUser(draft).tone?.traits).toHaveLength(toneTraitLimit);
	});

	test('an empty user document still saves a formal tone', () => {
		expect(draftToUser(userToDraft({ schemaVersion: 1 }))).toEqual({
			schemaVersion: 1,
			tone: { register: 'formal' }
		});
	});
});
