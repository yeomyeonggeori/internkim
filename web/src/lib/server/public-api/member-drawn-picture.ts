import { error, json } from '@sveltejs/kit';
import type { Environment } from '$lib/server/agent-request';
import type { CallingMember } from '$lib/server/member-request';
import { drawnPictureFormat, largestDrawnPictureBytes } from '$lib/profile/member-picture';
import { memberPictureKind } from './asset-address';
import {
	AssetStoreRefused,
	assetStoreCredentialsOf,
	dropFileFromTheBucket,
	keepFileInTheBucket,
	mayWriteAFile
} from './files';

const keepFunction = 'member_picture_keep_drawn';

export async function keepTheDrawnPicture(
	request: Request,
	environment: Environment,
	member: CallingMember
): Promise<Response> {
	if (!mayWriteAFile(member.permission)) {
		error(403, 'this token may only read, and keeping a picture is a write');
	}
	const picture = await drawnPictureOffered(request);
	if (picture.size === 0) error(400, 'this call carried no picture');
	if (picture.size > largestDrawnPictureBytes) {
		error(413, `a drawn picture is at most ${largestDrawnPictureBytes} bytes`);
	}
	if (picture.type.split(';')[0].trim().toLowerCase() !== drawnPictureFormat) {
		error(400, `a drawn picture is ${drawnPictureFormat}`);
	}

	if (await pictureOnTheRow(member)) return json({ kept: false }, { status: 200 });

	const credentials = assetStoreCredentialsOf(environment);
	const bytes = new Uint8Array(await picture.arrayBuffer());
	const kept = await keepFileInTheBucket(credentials, member.companyID, memberPictureKind, bytes, drawnPictureFormat).catch(
		(refusal: unknown) => {
			if (refusal instanceof AssetStoreRefused) error(502, refusal.message);
			throw refusal;
		}
	);

	const written = await member.caller.rpc(keepFunction, { picture_path: kept.path });
	if (written.error) {
		await dropFileFromTheBucket(credentials, kept.path).catch(() => undefined);
		error(422, written.error.message);
	}
	const isKept = written.data === true;
	if (!isKept && (await pictureOnTheRow(member)) !== kept.path) {
		await dropFileFromTheBucket(credentials, kept.path).catch(() => undefined);
	}
	return json({ kept: isKept }, { status: 200 });
}

async function drawnPictureOffered(request: Request): Promise<File> {
	const form = await request.formData().catch(() => null);
	const offered = form?.get('file');
	if (!(offered instanceof File)) {
		error(400, 'a drawn picture arrives as multipart/form-data in a file field');
	}
	return offered;
}

async function pictureOnTheRow(member: CallingMember): Promise<string> {
	const row = await member.caller
		.from('member')
		.select('profile_image')
		.eq('id', member.memberID)
		.single<{ profile_image: string | null }>();
	if (row.error) error(500, row.error.message);
	return row.data.profile_image ?? '';
}
