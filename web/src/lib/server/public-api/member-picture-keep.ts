import { error, json } from '@sveltejs/kit';
import type { Environment } from '$lib/server/agent-request';
import type { CallingMember } from '$lib/server/member-request';
import { isKeptPictureFormat, keptPictureFormats, largestKeptPictureBytes } from '$lib/profile/member-picture';
import { digestOf, memberPictureKind, sharedAssetPath } from './asset-address';
import {
	AssetStoreRefused,
	assetStoreCredentialsOf,
	dropFileFromTheBucket,
	keepFileInTheBucket,
	mayWriteAFile
} from './files';

const keepFunction = 'member_picture_keep';

export async function keepTheMemberPicture(
	request: Request,
	environment: Environment,
	member: CallingMember
): Promise<Response> {
	if (!mayWriteAFile(member.permission)) {
		error(403, 'this token may only read, and keeping a picture is a write');
	}
	const picture = await pictureOffered(request);
	if (picture.size === 0) error(400, 'this call carried no picture');
	if (picture.size > largestKeptPictureBytes) {
		error(413, `a member picture is at most ${largestKeptPictureBytes} bytes`);
	}
	const contentType = picture.type.split(';')[0].trim().toLowerCase();
	if (!isKeptPictureFormat(contentType)) {
		error(400, `a member picture is one of ${keptPictureFormats.join(', ')}`);
	}

	const bytes = new Uint8Array(await picture.arrayBuffer());
	const path = sharedAssetPath(member.companyID, memberPictureKind, await digestOf(bytes), contentType);
	if ((await pictureOnTheRow(member)) === path) return json({ kept: false }, { status: 200 });

	const credentials = assetStoreCredentialsOf(environment);
	const kept = await keepFileInTheBucket(credentials, member.companyID, memberPictureKind, bytes, contentType).catch(
		(refusal: unknown) => {
			if (refusal instanceof AssetStoreRefused) error(502, refusal.message);
			throw refusal;
		}
	);

	const written = await member.caller.rpc(keepFunction, { picture_path: path });
	if (written.error) {
		if (!kept.wasAlreadyKept) await dropFileFromTheBucket(credentials, path).catch(() => undefined);
		error(422, written.error.message);
	}
	const isKept = written.data === true;
	if (!isKept && !kept.wasAlreadyKept && (await pictureOnTheRow(member)) !== path) {
		await dropFileFromTheBucket(credentials, path).catch(() => undefined);
	}
	return json({ kept: isKept }, { status: 200 });
}

async function pictureOffered(request: Request): Promise<File> {
	const form = await request.formData().catch(() => null);
	const offered = form?.get('file');
	if (!(offered instanceof File)) {
		error(400, 'a member picture arrives as multipart/form-data in a file field');
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
