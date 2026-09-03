import type { AgentSoul, AgentToneRegister, AgentUser } from '../../routes/admin/admin-types';

export type ToneRegisterChoice = AgentToneRegister | '';

export type SoulDraft = {
	valuesText: string;
	boundariesText: string;
	workingStyleText: string;
	register: ToneRegisterChoice;
	traitsText: string;
	languageDefault: string;
	matchRequester: boolean;
};

export type UserDraft = {
	callMe: string;
	about: string;
	preferencesText: string;
	register: ToneRegisterChoice;
	traitsText: string;
	languageDefault: string;
};

export const toneRegisters: AgentToneRegister[] = ['formal', 'polite', 'casual'];

export function linesOf(values: string[] | undefined): string {
	return (values ?? []).join('\n');
}

export function listFrom(textValue: string): string[] {
	return textValue
		.split('\n')
		.map((line) => line.trim())
		.filter(Boolean);
}

export function soulToDraft(soul: AgentSoul): SoulDraft {
	return {
		valuesText: linesOf(soul.values),
		boundariesText: linesOf(soul.boundaries),
		workingStyleText: linesOf(soul.workingStyle),
		register: soul.tone?.register ?? '',
		traitsText: linesOf(soul.tone?.traits),
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
	const tone = toneFrom(draft.register, draft.traitsText);
	if (tone) soul.tone = tone;
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
		register: user.tone?.register ?? '',
		traitsText: linesOf(user.tone?.traits),
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
	const tone = toneFrom(draft.register, draft.traitsText);
	if (tone) user.tone = tone;
	const languageDefault = draft.languageDefault.trim();
	if (languageDefault) user.language = { default: languageDefault };
	return user;
}

function toneFrom(register: ToneRegisterChoice, traitsText: string): AgentSoul['tone'] | undefined {
	const traits = listFrom(traitsText);
	if (!register && !traits.length) return undefined;
	return { ...(register ? { register } : {}), ...(traits.length ? { traits } : {}) };
}
