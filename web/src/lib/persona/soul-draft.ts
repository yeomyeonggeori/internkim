import type { AgentToneRegister, AgentUser } from '../../routes/admin/admin-types';

export type UserDraft = {
	callMe: string;
	about: string;
	preferencesText: string;
	register: AgentToneRegister;
	traits: string[];
	languageDefault: string;
	morningBriefing?: AgentUser['morningBriefing'];
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

export function userToDraft(user: AgentUser): UserDraft {
	return {
		callMe: user.callMe ?? '',
		about: user.about ?? '',
		preferencesText: linesOf(user.preferences),
		register: user.tone?.register ?? defaultToneRegister,
		traits: traitTokensOf(user.tone?.traits),
		languageDefault: user.language?.default ?? '',
		morningBriefing: user.morningBriefing
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
	if (draft.morningBriefing) user.morningBriefing = draft.morningBriefing;
	return user;
}

function toneFrom(register: AgentToneRegister, traits: string[]): AgentUser['tone'] {
	const selectedTraits = traits.slice(0, toneTraitLimit);
	return { register, ...(selectedTraits.length ? { traits: selectedTraits } : {}) };
}
