import type { SupabaseClient } from '@supabase/supabase-js';
import { assetBucket } from '../asset-address';
import { companyImages } from '../catalog/company';
import { companyImagesKept } from './company-tools';

export type AnsweredFile = {
	name: string;
	mimeType: string;
	text?: string;
	bytes?: Uint8Array;
};

type FilesOfAnswer = (caller: SupabaseClient, body: unknown) => Promise<AnsweredFile[]>;

const filesOfTool: Record<string, FilesOfAnswer> = {
	company_info_get: companyProfileFiles
};

export async function filesAnsweredBy(toolName: string, caller: SupabaseClient, body: unknown): Promise<AnsweredFile[]> {
	const files = filesOfTool[toolName];
	return files ? files(caller, body) : [];
}

async function companyProfileFiles(caller: SupabaseClient, body: unknown): Promise<AnsweredFile[]> {
	const kept = await companyImagesKept(caller);
	const images: AnsweredFile[] = [];
	const printed: Record<string, unknown> = { ...profileIn(body) };
	for (const image of companyImages) {
		const path = kept[image];
		const file = path ? await storedImage(caller, path, `${image}${extensionOfPath(path)}`) : undefined;
		if (file) images.push(file);
		printed[`${image}Image`] = file?.name ?? '';
	}
	const profileFile = { name: 'company-profile.json', mimeType: 'application/json', text: JSON.stringify(printed, null, 2) };
	return [profileFile, ...images];
}

function profileIn(body: unknown): Record<string, unknown> {
	if (typeof body !== 'object' || body === null || !('result' in body)) {
		throw new Error('company_info_get answered no profile');
	}
	const { result } = body;
	if (typeof result !== 'object' || result === null) throw new Error('company_info_get answered no profile');
	return { ...result };
}

async function storedImage(caller: SupabaseClient, path: string, name: string): Promise<AnsweredFile> {
	const { data, error } = await caller.storage.from(assetBucket).download(path);
	if (error) throw new Error(`the company image at ${path} could not be read: ${error.message}`);
	return { name, mimeType: data.type || 'application/octet-stream', bytes: new Uint8Array(await data.arrayBuffer()) };
}

function extensionOfPath(path: string): string {
	const name = path.slice(path.lastIndexOf('/') + 1);
	const dot = name.lastIndexOf('.');
	return dot > 0 ? name.slice(dot).toLowerCase() : '';
}
