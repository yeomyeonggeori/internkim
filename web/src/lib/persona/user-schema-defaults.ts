import userSchema from '../../../../internal/admind/persona-schema/user.schema.json';

export type MorningBriefingSettings = {
	enabled: boolean;
	time: string;
};

export function morningBriefingDefaults(): MorningBriefingSettings {
	return { ...userSchema.properties.morningBriefing.default };
}
