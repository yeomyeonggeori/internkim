import { invokeTool } from '$lib/public-api-call';

export type CompanyWorkLocation = { name: string; color: string | null };

export type CompanySettings = {
	name: string;
	country: string;
	locale: string;
	timeZone: string;
	currencyCode: string;
	workLocations: CompanyWorkLocation[];
	leaveDays: number | null;
	profileImageURL: string | null;
};

export type CompanySettingsChange = {
	name?: string;
	locale?: string;
	timeZone?: string;
	currencyCode?: string;
	workLocations?: { name: string; color?: string }[];
	leaveDays?: number;
};

export function companySettings(): Promise<CompanySettings> {
	return invokeTool<CompanySettings>('company_settings_get', {});
}

export function saveCompanySettings(change: CompanySettingsChange): Promise<CompanySettings> {
	return invokeTool<CompanySettings>('company_settings_update', change);
}

export async function companyTimeZone(): Promise<string> {
	return (await invokeTool<CompanySettings>('company_settings_get', { includeProfileImage: false })).timeZone;
}
