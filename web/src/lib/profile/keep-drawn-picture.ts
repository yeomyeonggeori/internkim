import { drawnPictureOf } from '$lib/avatar-gradient/drawn-picture';
import { memberAccessToken } from '$lib/public-api-call';
import { drawnPictureFormat, memberPicturePath } from './member-picture';

const answeredKey = 'internkim.drawn-picture-answered';

export async function keepDrawnPicture(email: string): Promise<void> {
	const owner = email.trim().toLowerCase();
	if (!owner || wasAnsweredFor(owner)) return;

	const carried = new FormData();
	carried.set('file', new File([await drawnPictureOf(owner)], 'drawn.png', { type: drawnPictureFormat }));
	const response = await fetch(`/api/v1${memberPicturePath}`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${await memberAccessToken()}` },
		body: carried
	});
	if (!response.ok) {
		const said = (await response.text()).trim();
		throw new Error(said || `the member picture endpoint answered ${response.status}`);
	}
	rememberAnsweredFor(owner);
}

function wasAnsweredFor(owner: string): boolean {
	try {
		return localStorage.getItem(answeredKey) === owner;
	} catch {
		return false;
	}
}

function rememberAnsweredFor(owner: string): void {
	try {
		localStorage.setItem(answeredKey, owner);
	} catch {
		return;
	}
}
