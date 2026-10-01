import { z } from 'zod';
import { supabase } from '$lib/supabase';
export { sharedDataRoomSchema } from './schemas';

export async function dataRoomRequest(path: string, method = 'GET'): Promise<unknown> {
	const { data } = await supabase().auth.getSession();
	const headers = new Headers();
	if (data.session) headers.set('Authorization', `Bearer ${data.session.access_token}`);
	const response = await fetch(path, { method, headers });
	const answer: unknown = await response.json();
	if (!response.ok) {
		const refusal = z.object({ message: z.string() }).safeParse(answer);
		throw new Error(refusal.success ? refusal.data.message : `data room returned ${response.status}`);
	}
	return answer;
}

export async function dataRoomFile(companyID: string, documentID: string, fileName?: string): Promise<string> {
	const parameters = new URLSearchParams({ documentID });
	if (fileName) parameters.set('fileName', fileName);
	const answer = await dataRoomRequest(`/api/v1/data-room/${companyID}?${parameters}`);
	return z.object({ downloadURL: z.string().url() }).parse(answer).downloadURL;
}
