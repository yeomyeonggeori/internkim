import { createHash } from 'node:crypto';
import type { SupabaseClient } from '@supabase/supabase-js';
import { assetBucket } from './public-api/asset-address';
import { derivedPath, originalPath } from '../data-room/storage-path';

type UnfiledDocument = {
	id: string;
	company_id: string;
	title: string;
	document_type: string;
	category_code: string;
	sha256: string;
};

export type LegacyOriginal = {
	documentID: string;
	title: string;
	from: string;
	to: string;
	derived: { from: string; to: string }[];
};

export type LegacyOriginalsRestoration = {
	restored: LegacyOriginal[];
	unmatched: { documentID: string; title: string }[];
};

const legacyClearanceFolder = /^[0-9]+$/;
const zipSignature = [0x50, 0x4b, 0x03, 0x04];
const binarySignatures: [string, number[]][] = [
	['pdf', [0x25, 0x50, 0x44, 0x46]],
	['png', [0x89, 0x50, 0x4e, 0x47]],
	['jpg', [0xff, 0xd8, 0xff]],
	['gif', [0x47, 0x49, 0x46, 0x38]],
	['webp', [0x52, 0x49, 0x46, 0x46]]
];
const officeFolders: [string, string][] = [
	['pptx', 'ppt/'],
	['docx', 'word/'],
	['xlsx', 'xl/']
];

export async function restoreLegacyOriginals(
	client: SupabaseClient,
	options: { isDryRun: boolean }
): Promise<LegacyOriginalsRestoration> {
	const restoration: LegacyOriginalsRestoration = { restored: [], unmatched: [] };
	for (const document of await unfiledDocuments(client)) {
		const legacy = await legacyOriginalOf(client, document);
		if (!legacy) {
			restoration.unmatched.push({ documentID: document.id, title: document.title });
			continue;
		}
		if (!options.isDryRun) await refile(client, document.id, legacy);
		restoration.restored.push(legacy);
	}
	return restoration;
}

export function extensionOfBytes(bytes: Uint8Array): string {
	const signed = binarySignatures.find(([, signature]) => startsWith(bytes, signature));
	if (signed) return signed[0];
	if (startsWith(bytes, zipSignature)) return officeExtensionOf(bytes);
	const opening = new TextDecoder().decode(bytes.subarray(0, 512)).trimStart().toLowerCase();
	if (opening.startsWith('<!doctype html') || opening.startsWith('<html')) return 'html';
	if (opening.startsWith('<svg')) return 'svg';
	return opening.startsWith('#') ? 'md' : 'txt';
}

function startsWith(bytes: Uint8Array, signature: number[]): boolean {
	return signature.every((byte, index) => bytes[index] === byte);
}

function officeExtensionOf(bytes: Uint8Array): string {
	const archive = new TextDecoder('latin1').decode(bytes);
	return officeFolders.find(([, folder]) => archive.includes(folder))?.[0] ?? 'zip';
}

async function unfiledDocuments(client: SupabaseClient): Promise<UnfiledDocument[]> {
	const { data, error } = await client
		.from('company_document')
		.select('id, company_id, title, document_type, category_code, sha256')
		.is('storage_path', null)
		.not('sha256', 'is', null)
		.order('created_at')
		.returns<UnfiledDocument[]>();
	if (error) throw new Error(`documents without an original: ${error.message}`);
	return data;
}

async function legacyOriginalOf(client: SupabaseClient, document: UnfiledDocument): Promise<LegacyOriginal | undefined> {
	for (const folder of await legacyFoldersOf(client, document.company_id)) {
		const from = `${folder}/${document.sha256}`;
		const bytes = await downloadIfPresent(client, from);
		if (!bytes || sha256Of(bytes) !== document.sha256) continue;
		const to = originalPath(
			{ companyID: document.company_id, categoryCode: document.category_code, documentID: document.id },
			`${document.title}.${extensionOfBytes(bytes)}`,
			[document.document_type]
		);
		const derived = (await filesUnder(client, from)).map((name) => ({ from: `${from}/${name}`, to: derivedPath(to, document.id, name) }));
		return { documentID: document.id, title: document.title, from, to, derived };
	}
	return undefined;
}

async function legacyFoldersOf(client: SupabaseClient, companyID: string): Promise<string[]> {
	const { data, error } = await client.storage.from(assetBucket).list(`${companyID}/dataroom`);
	if (error) throw new Error(`data room folders of ${companyID}: ${error.message}`);
	return data
		.filter((entry) => entry.id === null && legacyClearanceFolder.test(entry.name))
		.map((entry) => `${companyID}/dataroom/${entry.name}`);
}

async function filesUnder(client: SupabaseClient, folder: string): Promise<string[]> {
	const { data, error } = await client.storage.from(assetBucket).list(folder);
	if (error) throw new Error(`files under ${folder}: ${error.message}`);
	return data.filter((entry) => entry.id !== null).map((entry) => entry.name);
}

async function downloadIfPresent(client: SupabaseClient, path: string): Promise<Uint8Array | undefined> {
	const { data } = await client.storage.from(assetBucket).download(path);
	return data ? new Uint8Array(await data.arrayBuffer()) : undefined;
}

function sha256Of(bytes: Uint8Array): string {
	return createHash('sha256').update(bytes).digest('hex');
}

async function refile(client: SupabaseClient, documentID: string, legacy: LegacyOriginal): Promise<void> {
	const moves = [{ from: legacy.from, to: legacy.to }, ...legacy.derived];
	for (const move of moves) await moveObject(client, move.from, move.to);
	const { error } = await client
		.from('company_document')
		.update({ storage_path: legacy.to })
		.eq('id', documentID)
		.is('storage_path', null);
	if (!error) return;
	for (const move of moves) await moveObject(client, move.to, move.from);
	throw new Error(`refiling ${documentID}: ${error.message}`);
}

async function moveObject(client: SupabaseClient, from: string, to: string): Promise<void> {
	const { error } = await client.storage.from(assetBucket).move(from, to);
	if (error) throw new Error(`moving ${from} to ${to}: ${error.message}`);
}
