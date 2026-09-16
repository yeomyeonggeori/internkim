import { drawnPictureOf } from '$lib/avatar-gradient/drawn-picture';
import { externalIDFor, fetchMessengerDirectory } from '$lib/messenger/messenger-directory';
import { memberAccessToken } from '$lib/public-api-call';
import { personPicture } from '$lib/stores/person-picture.svelte';
import { acceptedMemberPicture, memberPicturePath } from './member-picture';

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
	await personPicture.rememberExternals([externalID]);
	const readable = personPicture.pictureOfExternal(externalID);
	if (!readable) return null;
	const response = await fetch(readable);
	if (!response.ok) throw new Error(`the kept messenger picture answered ${response.status}`);
	return acceptedMemberPicture(await response.blob());
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
