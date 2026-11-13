import { z } from 'zod';
import { supabase } from '$lib/supabase';
import { DataRoomRefused } from './viewer';
export { sharedDataRoomSchema } from './schemas';

export async function dataRoomRequest(path: string, method = 'GET'): Promise<unknown> {
	const { data } = await supabase().auth.getSession();
	const headers = new Headers();
	if (data.session) headers.set('Authorization', `Bearer ${data.session.access_token}`);
	return answerOf(await fetch(path, { method, headers }));
}

export async function publicDataRoomRead(path: string): Promise<unknown> {
	return answerOf(await fetch(path));
}

async function answerOf(response: Response): Promise<unknown> {
	const answer: unknown = await response.json();
	if (!response.ok) {
		const refusal = z.object({ message: z.string() }).safeParse(answer);
		throw new DataRoomRefused(refusal.success ? refusal.data.message : `data room returned ${response.status}`, response.status);
	}
	return answer;
}

export async function savedDownload(response: Response, fileName: string): Promise<void> {
	if (!response.ok) {
		const refusal = z.object({ message: z.string() }).safeParse(await response.json().catch(() => null));
		throw new DataRoomRefused(refusal.success ? refusal.data.message : `data room returned ${response.status}`, response.status);
	}
	const address = URL.createObjectURL(await response.blob());
	const link = document.createElement('a');
	link.href = address;
	link.download = fileName;
	link.click();
	setTimeout(() => URL.revokeObjectURL(address), 60_000);
}

export async function publicDataRoomDownload(companyID: string, documentID: string, fileName: string): Promise<void> {
	const parameters = new URLSearchParams({ documentID, download: '1' });
	await savedDownload(await fetch(`/api/v1/data-room/${companyID}?${parameters}`), fileName);
}

export async function publicDataRoomFile(companyID: string, documentID: string, fileName?: string): Promise<string> {
	const parameters = new URLSearchParams({ documentID });
	if (fileName) parameters.set('fileName', fileName);
	const answer = await publicDataRoomRead(`/api/v1/data-room/${companyID}?${parameters}`);
	return z.object({ downloadURL: z.string().url() }).parse(answer).downloadURL;
}

export async function publicDataRoomCompanyOf(slug: string): Promise<string> {
	const answer = await publicDataRoomRead(`/api/v1/data-room/public/${encodeURIComponent(slug)}`);
	return z.object({ companyID: z.guid() }).parse(answer).companyID;
}
