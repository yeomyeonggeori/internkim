import type { SupabaseClient } from '@supabase/supabase-js';
import {
	companyPictureFormats,
	companyPictureMegabytes,
	isCompanyPictureFormat,
	refusalOfCompanyPictureSize
} from '$lib/company/company-picture';
import { assetBucket, companyPictureKind } from './asset-address';
import { InputRefused, AccessRefused, RecordMoved, ServiceUnanswered, refusalOfPostgresError } from './refusal';

const readableForOneHour = 60 * 60;

export function companyPictureFolder(companyID: string): string {
	return `${companyID}/shared/${companyPictureKind}`;
}

export async function companyPictureUploadPlace(
	caller: SupabaseClient,
	companyID: string,
	extension: string
): Promise<{ storagePath: string; uploadURL: string }> {
	const storagePath = `${companyPictureFolder(companyID)}/upload-${crypto.randomUUID()}.${extension}`;
	const { data, error } = await caller.storage.from(assetBucket).createSignedUploadUrl(storagePath);
	if (error) throw new AccessRefused(`only an administrator can change the company picture: ${error.message}`);
	return { storagePath, uploadURL: data.signedUrl };
}

export async function keepUploadedCompanyPicture(
	caller: SupabaseClient,
	companyID: string,
	storagePath: string
): Promise<string | null> {
	const folder = companyPictureFolder(companyID);
	if (!storagePath.startsWith(`${folder}/`)) {
		throw new InputRefused("profileImage is the storagePath company_image_upload answered for image 'picture'");
	}
	await refuseUnlessAPictureIsThere(caller, folder, storagePath);
	return companyPictureWritten(caller, storagePath);
}

async function refuseUnlessAPictureIsThere(caller: SupabaseClient, folder: string, storagePath: string): Promise<void> {
	const name = storagePath.slice(folder.length + 1);
	const { data, error } = await caller.storage.from(assetBucket).list(folder, { search: name });
	if (error) throw new ServiceUnanswered(error.message);
	const held = (data ?? []).find((entry) => entry.name === name);
	if (!held) {
		throw new RecordMoved(`nothing is kept at ${storagePath} yet: PUT the picture to the uploadURL company_image_upload answered first`);
	}
	const tooBig = refusalOfCompanyPictureSize(Number(held.metadata?.size ?? 0));
	if (tooBig) {
		throw new InputRefused(`this picture is ${tooBig}, over the ${companyPictureMegabytes}MB a company picture may be`);
	}
	if (!isCompanyPictureFormat(String(held.metadata?.mimetype ?? ''))) {
		throw new InputRefused(`a company picture is one of ${companyPictureFormats.join(', ')}, sent as the Content-Type of the PUT`);
	}
}

export async function companyPictureWritten(caller: SupabaseClient, path: string | null): Promise<string | null> {
	const company = await caller.from('company').select('id').limit(1).single<{ id: string }>();
	if (company.error) throw new Error(company.error.message);

	const written = await caller
		.from('company')
		.update({ profile_image: path })
		.eq('id', company.data.id)
		.select('id');
	if (written.error) throw refusalOfPostgresError(written.error);
	if ((written.data ?? []).length === 0) {
		throw new AccessRefused('only an administrator can change the company picture');
	}
	if (!path) return null;

	const signed = await caller.storage.from(assetBucket).createSignedUrl(path, readableForOneHour);
	if (signed.error) throw new ServiceUnanswered(signed.error.message);
	return signed.data?.signedUrl ?? null;
}
