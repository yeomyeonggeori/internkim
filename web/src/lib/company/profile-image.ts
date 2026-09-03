import { memberAccessToken } from '$lib/public-api-call';
import { companyPicturePath, refusalOfCompanyPictureSize } from './company-picture';
import { companySettings } from './company-settings';
import { isSupabaseConfigured } from '$lib/supabase';

export type CompanyProfileImage = { name: string; readableURL: string };

export function refusalOfCompanyPicture(file: File): string {
	return refusalOfCompanyPictureSize(file.size);
}

export async function loadCompanyProfileImage(): Promise<CompanyProfileImage> {
	if (!isSupabaseConfigured()) return { name: '', readableURL: '' };
	const settings = await companySettings();
	return { name: settings.name, readableURL: settings.profileImageURL ?? '' };
}

export async function saveCompanyProfileImage(file: File): Promise<string> {
	const carried = new FormData();
	carried.set('file', file);
	return (await askForTheCompanyPicture('POST', carried)).profileImageURL ?? '';
}

export async function forgetCompanyProfileImage(): Promise<void> {
	await askForTheCompanyPicture('DELETE');
}

async function askForTheCompanyPicture(
	method: string,
	body?: FormData
): Promise<{ profileImageURL: string | null }> {
	const response = await fetch(`/api/v1${companyPicturePath}`, {
		method,
		headers: { Authorization: `Bearer ${await memberAccessToken()}` },
		...(body ? { body } : {})
	});
	if (!response.ok) {
		const said = (await response.text()).trim();
		throw new Error(said || `the company picture endpoint answered ${response.status}`);
	}
	return await response.json();
}
