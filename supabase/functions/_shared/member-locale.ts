import type { SupabaseClient } from './service-client.ts';

export type Locale = 'ko' | 'en';

export function localeOf(tag: string | null | undefined): Locale {
	return (tag ?? '').trim().toLowerCase().startsWith('en') ? 'en' : 'ko';
}

export async function localeOfMember(client: SupabaseClient, memberID: string): Promise<Locale> {
	const { data, error } = await client
		.from('member')
		.select('locale, company(locale)')
		.eq('id', memberID)
		.maybeSingle<{ locale: string | null; company: { locale: string } | null }>();
	if (error) throw new Error(`the language of member ${memberID} could not be read: ${error.message}`);
	return localeOf(data?.locale ?? data?.company?.locale);
}
