import { dataRoomFolder } from './storage-path';

export const serviceFiles = [
	{ name: 'seal', categoryCode: 'CR', title: 'Company seal', purpose: 'the seal stamped on forms' },
	{ name: 'logo', categoryCode: 'SM', title: 'Company logo', purpose: 'the logo printed on letterheads' }
] as const;

export type ServiceFile = (typeof serviceFiles)[number];

export type ServiceFileName = ServiceFile['name'];

export type ServiceFileImageField = `${ServiceFileName}Image`;

export const serviceFileNames = serviceFiles.map((file) => file.name);

export function imageFieldOf(name: ServiceFileName): ServiceFileImageField {
	return `${name}Image`;
}

export const serviceFileImageExtensions = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'avif', 'heic', 'heif', 'tif', 'tiff'];

export type ServiceFileVersion = {
	name: ServiceFileName;
	date: string;
	documentID: string;
	extension: string;
};

const versionPattern =
	/^([a-z]+)\.(\d{4}-\d{2}-\d{2})\.([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.([a-z0-9]{1,10})$/;

export function serviceFileNamed(name: string | undefined): ServiceFile | undefined {
	return serviceFiles.find((file) => file.name === name?.trim());
}

export function serviceFilePath(companyID: string, version: ServiceFileVersion): string {
	const file = serviceFiles.find((candidate) => candidate.name === version.name);
	if (!file) throw new Error(`${version.name} is not a service file`);
	return `${dataRoomFolder(companyID, file.categoryCode)}/${version.name}.${version.date}.${version.documentID}.${version.extension}`;
}

export function serviceFileVersionAt(companyID: string, path: string): ServiceFileVersion | undefined {
	const fileName = path.slice(path.lastIndexOf('/') + 1);
	const [, name, date, documentID, extension] = versionPattern.exec(fileName) ?? [];
	const file = serviceFileNamed(name);
	if (!file || !date || !documentID || !extension) return undefined;
	const version = { name: file.name, date, documentID, extension };
	return serviceFilePath(companyID, version) === path ? version : undefined;
}
