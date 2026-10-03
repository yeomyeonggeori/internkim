import type { SupabaseClient } from '@supabase/supabase-js';
import { assetBucket } from '../asset-address';
import { serviceFileNames } from '$lib/data-room/service-files';
import { newestServiceFile, type KeptServiceFile } from './service-files';

export type AnsweredFile = {
	name: string;
	mimeType: string;
	text?: string;
	bytes?: Uint8Array;
};

export type AnsweringCompany = { caller: SupabaseClient; companyID: string };

type FilesOfAnswer = (company: AnsweringCompany, body: unknown) => Promise<AnsweredFile[]>;

const filesOfTool: Record<string, FilesOfAnswer> = {
	company_info_get: companyProfileFiles
};

export async function filesAnsweredBy(toolName: string, company: AnsweringCompany, body: unknown): Promise<AnsweredFile[]> {
	const files = filesOfTool[toolName];
	return files ? files(company, body) : [];
}

async function companyProfileFiles(company: AnsweringCompany, body: unknown): Promise<AnsweredFile[]> {
	const images: AnsweredFile[] = [];
	const printed: Record<string, unknown> = { ...profileIn(body) };
	for (const name of serviceFileNames) {
		const kept = await newestServiceFile(company.caller, company.companyID, name);
		const file = kept ? await storedImage(company.caller, kept) : undefined;
		if (file) images.push(file);
		printed[`${name}Image`] = file?.name ?? '';
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

async function storedImage(caller: SupabaseClient, kept: KeptServiceFile): Promise<AnsweredFile | undefined> {
	const { data, error } = await caller.storage.from(assetBucket).download(kept.storagePath);
	if (error && isWithheldFromTheCaller(error)) return undefined;
	if (error) throw new Error(`the company image at ${kept.storagePath} could not be read: ${error.message}`);
	return {
		name: `${kept.name}.${kept.extension}`,
		mimeType: data.type || 'application/octet-stream',
		bytes: new Uint8Array(await data.arrayBuffer())
	};
}

function isWithheldFromTheCaller(error: Error): boolean {
	return 'status' in error && (error.status === 400 || error.status === 403 || error.status === 404);
}
