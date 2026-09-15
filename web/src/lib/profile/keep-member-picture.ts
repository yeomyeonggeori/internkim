import { drawnPictureOf } from '$lib/avatar-gradient/drawn-picture';
import { fetchProfilePicture } from '$lib/messenger/messenger-api';
import { externalIDFor, fetchMessengerDirectory } from '$lib/messenger/messenger-directory';
import { memberAccessToken } from '$lib/public-api-call';
import { isKeptPictureFormat, memberPicturePath } from './member-picture';

const answeredKey = 'internkim.member-picture-answered';

export async function keepMemberPicture(email: string): Promise<void> {
	const owner = email.trim().toLowerCase();
	if (!owner) return;

	const picture = (await messengerPictureOf(owner)) ?? (await drawnPictureOf(owner));
	const answer = `${owner}:${await fingerprintOf(picture)}`;
	if (wasAnswered(answer)) return;

	const carried = new FormData();
	carried.set('file', new File([picture], 'picture', { type: picture.type }));
	const response = await fetch(`/api/v1${memberPicturePath}`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${await memberAccessToken()}` },
		body: carried
	});
	if (!response.ok) {
		const said = (await response.text()).trim();
		throw new Error(said || `the member picture endpoint answered ${response.status}`);
	}
	rememberAnswered(answer);
}

async function messengerPictureOf(owner: string): Promise<Blob | null> {
	const externalID = externalIDFor({ email: owner }, await fetchMessengerDirectory());
	if (!externalID) return null;
	const picture = await fetchProfilePicture(externalID);
	return picture ? blobOfDataURL(picture.dataURL) : null;
}

export function blobOfDataURL(dataURL: string): Blob {
	const parsed = /^data:([^;,]+);base64,(.+)$/.exec(dataURL);
	if (!parsed) throw new Error('the messenger answered a picture that is not a base64 data url');
	if (!isKeptPictureFormat(parsed[1])) {
		throw new Error(`the messenger picture is ${parsed[1]}, which a member picture cannot be`);
	}
	const bytes = Uint8Array.from(atob(parsed[2]), (character) => character.charCodeAt(0));
	return new Blob([bytes], { type: parsed[1] });
}

async function fingerprintOf(picture: Blob): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', await picture.arrayBuffer());
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

function wasAnswered(answer: string): boolean {
	try {
		return localStorage.getItem(answeredKey) === answer;
	} catch {
		return false;
	}
}

function rememberAnswered(answer: string): void {
	try {
		localStorage.setItem(answeredKey, answer);
	} catch {
		return;
	}
}
