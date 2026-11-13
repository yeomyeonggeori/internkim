import userSchema from '../../../../internal/admind/persona-schema/user.schema.json';

export type MorningBriefingSettings = {
	enabled: boolean;
	time: string;
};

export type WeeklyReportDay = (typeof userSchema.properties.weeklyReport.properties.day.enum)[number];

export type WeeklyReportSettings = {
	enabled: boolean;
	day: string;
	time: string;
};

export const weeklyReportDays = userSchema.properties.weeklyReport.properties.day.enum;

export function morningBriefingDefaults(): MorningBriefingSettings {
	return { ...userSchema.properties.morningBriefing.default };
}

export function weeklyReportDefaults(): WeeklyReportSettings {
	return { ...userSchema.properties.weeklyReport.default };
}
