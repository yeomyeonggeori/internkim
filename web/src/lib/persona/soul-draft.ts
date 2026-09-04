import type { AgentSoul, AgentToneRegister, AgentUser } from '../../routes/admin/admin-types';

export type SoulDraft = {
	valuesText: string;
	boundariesText: string;
	workingStyleText: string;
	register: AgentToneRegister;
	traits: string[];
	languageDefault: string;
	matchRequester: boolean;
};

export type UserDraft = {
	callMe: string;
	about: string;
	preferencesText: string;
	register: AgentToneRegister;
	traits: string[];
	languageDefault: string;
};

export const toneRegisters: AgentToneRegister[] = ['formal', 'polite', 'casual'];

export const defaultToneRegister: AgentToneRegister = 'formal';

export const toneTraits = ['calm', 'warm', 'clear', 'concise', 'direct', 'playful', 'meticulous', 'energetic'];

export const toneTraitLimit = 6;

const legacyTraitTokens: Record<string, string> = {
	차분한: 'calm',
	따뜻한: 'warm',
	또렷한: 'clear',
	간결한: 'concise'
};

export function linesOf(values: string[] | undefined): string {
	return (values ?? []).join('\n');
}

export function listFrom(textValue: string): string[] {
	return textValue
		.split('\n')
		.map((line) => line.trim())
		.filter(Boolean);
}

function traitTokensOf(traits: string[] | undefined): string[] {
	return (traits ?? []).map((trait) => legacyTraitTokens[trait] ?? trait);
}

export function soulToDraft(soul: AgentSoul): SoulDraft {
	return {
		valuesText: linesOf(soul.values),
		boundariesText: linesOf(soul.boundaries),
		workingStyleText: linesOf(soul.workingStyle),
		register: soul.tone?.register ?? defaultToneRegister,
		traits: traitTokensOf(soul.tone?.traits),
		languageDefault: soul.language?.default ?? '',
		matchRequester: soul.language?.matchRequester ?? false
	};
}

export function draftToSoul(draft: SoulDraft): AgentSoul {
	const soul: AgentSoul = { schemaVersion: 1 };
	const values = listFrom(draft.valuesText);
	const boundaries = listFrom(draft.boundariesText);
	const workingStyle = listFrom(draft.workingStyleText);
	if (values.length) soul.values = values;
	if (boundaries.length) soul.boundaries = boundaries;
	if (workingStyle.length) soul.workingStyle = workingStyle;
	soul.tone = toneFrom(draft.register, draft.traits);
	const languageDefault = draft.languageDefault.trim();
	if (languageDefault || draft.matchRequester) {
		soul.language = { ...(languageDefault ? { default: languageDefault } : {}), matchRequester: draft.matchRequester };
	}
	return soul;
}

export function userToDraft(user: AgentUser): UserDraft {
	return {
		callMe: user.callMe ?? '',
		about: user.about ?? '',
		preferencesText: linesOf(user.preferences),
		register: user.tone?.register ?? defaultToneRegister,
		traits: traitTokensOf(user.tone?.traits),
		languageDefault: user.language?.default ?? ''
	};
}

export function draftToUser(draft: UserDraft): AgentUser {
	const user: AgentUser = { schemaVersion: 1 };
	const callMe = draft.callMe.trim();
	const about = draft.about.trim();
	const preferences = listFrom(draft.preferencesText);
	if (callMe) user.callMe = callMe;
	if (about) user.about = about;
	if (preferences.length) user.preferences = preferences;
	user.tone = toneFrom(draft.register, draft.traits);
	const languageDefault = draft.languageDefault.trim();
	if (languageDefault) user.language = { default: languageDefault };
	return user;
}

function toneFrom(register: AgentToneRegister, traits: string[]): AgentSoul['tone'] {
	const selectedTraits = traits.slice(0, toneTraitLimit);
	return { register, ...(selectedTraits.length ? { traits: selectedTraits } : {}) };
}
