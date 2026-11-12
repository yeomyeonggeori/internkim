import type { SupabaseClient } from '@supabase/supabase-js';
import { assetBucket } from '../asset-address';
import { companyPictureImage, largestPictureACompanyCanHave } from '$lib/company/company-picture';
import { dataRoomFolder } from '$lib/data-room/storage-path';
import {
	imageFieldOf,
	serviceFileImageExtensions,
	serviceFileNamed,
	serviceFileNames,
	serviceFilePath,
	serviceFileVersionAt,
	type ServiceFile,
	type ServiceFileImageField,
	type ServiceFileName,
	type ServiceFileVersion
} from '$lib/data-room/service-files';
import { dayIn } from './days';
import type { RecordContext } from './company';
import type { CompanyImageUploadResult } from '../catalog/company';
import { companyPictureUploadPlace } from '../company-picture-keep';
import { Refusal, InputRefused, AccessRefused, RecordMoved, refusalOfPostgresError, refusalOfServiceError, type PostgresError } from '../refusal';

export type CompanyImageUploadInput = { image?: string; fileName?: string; date?: string };

export type KeptServiceFile = ServiceFileVersion & { storagePath: string };

type VersionRow = { id: string; storage_path: string | null };

const onlyAnAdministratorKeepsServiceFiles = 'only an administrator may keep a company image';

export async function newestServiceFile(
	caller: SupabaseClient,
	companyID: string,
	name: ServiceFileName
): Promise<KeptServiceFile | undefined> {
	const file = serviceFileOf(name);
	const { data, error } = await caller
		.from('company_document')
		.select('id, storage_path')
		.eq('company_id', companyID)
		.eq('category_code', file.categoryCode)
		.like('storage_path', `${dataRoomFolder(companyID, file.categoryCode)}/${file.name}.%`)
		.order('document_date', { ascending: false })
		.order('created_at', { ascending: false })
		.returns<VersionRow[]>();
	if (error) throw refusalOfPostgresError(error);
	return (data ?? []).map((row) => keptVersionOf(companyID, row)).find((version) => version !== undefined);
}

function keptVersionOf(companyID: string, row: VersionRow): KeptServiceFile | undefined {
	const version = row.storage_path ? serviceFileVersionAt(companyID, row.storage_path) : undefined;
	if (!version || !row.storage_path || version.documentID !== row.id) return undefined;
	return { ...version, storagePath: row.storage_path };
}

export async function companyImageUpload(
	context: RecordContext,
	input: CompanyImageUploadInput
): Promise<CompanyImageUploadResult> {
	if (input.image?.trim() === companyPictureImage) {
		return companyPictureUploadPlace(context.caller, context.companyID, imageExtensionOf(input.fileName));
	}
	const file = serviceFileOfInput(input.image);
	const extension = imageExtensionOf(input.fileName);
	const date = effectiveDateOf(input.date, context);
	await refuseAnyoneButAnAdministrator(context);
	const documentID = await pendingVersion(context, file, date);
	const storagePath = serviceFilePath(context.companyID, { name: file.name, date, documentID, extension });
	const { data, error } = await context.caller.storage.from(assetBucket).createSignedUploadUrl(storagePath);
	if (error) throw refusalOfServiceError(error);
	return { storagePath, uploadURL: data.signedUrl };
}

export async function keepUploadedImages(
	context: RecordContext,
	offered: Partial<Record<ServiceFileImageField, string>>
): Promise<void> {
	for (const name of serviceFileNames) {
		const storagePath = offered[imageFieldOf(name)];
		if (storagePath !== undefined) await keepUploadedImage(context, name, storagePath.trim());
	}
}

async function keepUploadedImage(context: RecordContext, name: ServiceFileName, storagePath: string): Promise<void> {
	const version = serviceFileVersionAt(context.companyID, storagePath);
	if (version?.name !== name) {
		throw new InputRefused(`${imageFieldOf(name)} is the storagePath company_image_upload answered for image '${name}'; a kept version stays, so none is removed`);
	}
	await refuseUnlessUploaded(context, storagePath);
	const previous = await newestServiceFile(context.caller, context.companyID, version.name);
	if (previous?.documentID === version.documentID) return;
	const supersedes = previous && previous.date <= version.date ? previous.documentID : null;
	const { data, error } = await context.caller
		.from('company_document')
		.update({ storage_path: storagePath, supersedes })
		.eq('id', version.documentID)
		.is('storage_path', null)
		.select('id');
	if (error) throw refusalOfTheWrite(error);
	if ((data ?? []).length === 0) {
		throw new RecordMoved(`${storagePath} names no company image waiting to be kept`);
	}
}

function serviceFileOf(name: ServiceFileName): ServiceFile {
	const file = serviceFileNamed(name);
	if (!file) throw new Error(`${name} is not a service file`);
	return file;
}

function serviceFileOfInput(offered: string | undefined): ServiceFile {
	const file = serviceFileNamed(offered);
	if (!file) throw new InputRefused(`image is one of ${[...serviceFileNames, companyPictureImage].join(', ')}`);
	return file;
}

function imageExtensionOf(fileName: string | undefined): string {
	const written = fileName?.trim().toLowerCase() ?? '';
	const extension = written.slice(written.lastIndexOf('.') + 1);
	if (!written.includes('.') || !serviceFileImageExtensions.includes(extension)) {
		throw new InputRefused(`a company image is a ${serviceFileImageExtensions.join(', ')} file`);
	}
	return extension;
}

function effectiveDateOf(offered: string | undefined, context: RecordContext): string {
	const date = offered?.trim() || dayIn(context.labels.timezone, context.now);
	if (!isCalendarDate(date)) throw new InputRefused('date is a calendar day in YYYY-MM-DD format');
	return date;
}

function isCalendarDate(date: string): boolean {
	if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return false;
	const instant = new Date(`${date}T00:00:00Z`);
	return !Number.isNaN(instant.getTime()) && instant.toISOString().startsWith(date);
}

async function refuseAnyoneButAnAdministrator(context: RecordContext): Promise<void> {
	const { data, error } = await context.caller.rpc('data_room_administrator', { target_company: context.companyID });
	if (error) throw refusalOfPostgresError(error);
	if (data !== true) throw new AccessRefused(onlyAnAdministratorKeepsServiceFiles);
}

async function pendingVersion(context: RecordContext, file: ServiceFile, date: string): Promise<string> {
	const { data, error } = await context.caller
		.from('company_document')
		.insert({
			company_id: context.companyID,
			kind: 'internal',
			document_type: file.name,
			title: file.title,
			summary: `${file.title}, in effect from ${date}.`,
			requester_id: context.requesterID,
			category_code: file.categoryCode,
			document_date: date
		})
		.select('id')
		.single<{ id: string }>();
	if (error) throw refusalOfTheWrite(error);
	return data.id;
}

async function refuseUnlessUploaded(context: RecordContext, storagePath: string): Promise<void> {
	const folder = storagePath.slice(0, storagePath.lastIndexOf('/'));
	const name = storagePath.slice(folder.length + 1);
	const { data, error } = await context.caller.storage.from(assetBucket).list(folder, { search: name });
	if (error) throw refusalOfServiceError(error);
	const held = (data ?? []).find((entry) => entry.name === name);
	if (!held) {
		throw new RecordMoved(`nothing is kept at ${storagePath} yet: PUT the image to the uploadURL company_image_upload answered first`);
	}
	const sizeBytes = Number(held.metadata?.size ?? 0);
	if (sizeBytes > largestPictureACompanyCanHave) {
		throw new InputRefused(`this image is ${sizeBytes} bytes, over the ${largestPictureACompanyCanHave} a company image may be`);
	}
}

function refusalOfTheWrite(error: PostgresError): Refusal {
	return refusalOfPostgresError(error, onlyAnAdministratorKeepsServiceFiles);
}

