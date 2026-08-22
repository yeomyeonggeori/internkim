import type { SupabaseClient } from '@supabase/supabase-js';
import { assetBucket } from '$lib/messenger/kept-attachment';
import { isSupabaseConfigured, supabase } from '$lib/supabase';

export type CompanyProfileImage = { companyID: string; path: string; readableURL: string };

type CompanyProfileImageRow = { id: string; profile_image: string | null };

async function readCompanyProfileImage(client: SupabaseClient): Promise<CompanyProfileImageRow> {
	const company = await client
		.from('company')
		.select('id, profile_image')
		.limit(1)
		.single<CompanyProfileImageRow>();
	if (company.error) throw new Error(company.error.message);
	return company.data;
}

// The bucket is private, so a path is only a picture once somebody signs it. The
// signature is what expires; the path in the row does not.
async function readableURLOf(client: SupabaseClient, path: string): Promise<string> {
	if (!path) return '';
	const signed = await client.storage.from(assetBucket).createSignedUrl(path, 60 * 60);
	if (signed.error) throw new Error(signed.error.message);
	return signed.data?.signedUrl ?? '';
}

export async function loadCompanyProfileImage(client: SupabaseClient = supabase()): Promise<CompanyProfileImage> {
	if (!isSupabaseConfigured()) return { companyID: '', path: '', readableURL: '' };
	const company = await readCompanyProfileImage(client);
	const path = company.profile_image ?? '';
	return { companyID: company.id, path, readableURL: await readableURLOf(client, path) };
}

// The path says which company the picture belongs to and who may read it, and a
// check on the row refuses one that says anything else. A name of its own keeps
// a new picture from being served out of the browser's copy of the old one.
function pathFor(companyID: string, file: File): string {
	const extension = (file.name.split('.').pop() ?? 'png').toLowerCase().replace(/[^a-z0-9]/g, '');
	return `${companyID}/shared/company/${crypto.randomUUID()}.${extension || 'png'}`;
}

export async function saveCompanyProfileImage(
	file: File,
	client: SupabaseClient = supabase()
): Promise<CompanyProfileImage> {
	const company = await readCompanyProfileImage(client);
	const path = pathFor(company.id, file);

	const uploaded = await client.storage.from(assetBucket).upload(path, file, { contentType: file.type });
	if (uploaded.error) throw new Error(uploaded.error.message);

	const saved = await client.from('company').update({ profile_image: path }).eq('id', company.id).select('id');
	if (saved.error) throw new Error(saved.error.message);
	// An administrator is the only one the row lets through, and a refusal reads
	// as no rows rather than an error. The picture would otherwise sit in the
	// bucket belonging to nothing.
	if ((saved.data ?? []).length === 0) {
		await client.storage.from(assetBucket).remove([path]);
		throw new Error('only an administrator can change the company picture');
	}
	return { companyID: company.id, path, readableURL: await readableURLOf(client, path) };
}

export async function forgetCompanyProfileImage(client: SupabaseClient = supabase()): Promise<void> {
	const company = await readCompanyProfileImage(client);
	const saved = await client.from('company').update({ profile_image: null }).eq('id', company.id).select('id');
	if (saved.error) throw new Error(saved.error.message);
	if ((saved.data ?? []).length === 0) throw new Error('only an administrator can change the company picture');
}
