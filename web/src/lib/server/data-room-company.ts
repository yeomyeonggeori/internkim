import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { storedCompanySchema, type DataRoomCompany } from '$lib/data-room/company';
import type { CompanyActivity, StoredActivity } from '$lib/data-room/activity';
import { controlPlane, type ControlPlaneCredentials } from './control-plane';

const thumbnailSide = 8;
const largestThumbnailBytes = 4096;

type Answer = { data: unknown; error: { message: string } | null };

export async function companyShownTo(caller: SupabaseClient, companyID: string, plane: ControlPlaneCredentials): Promise<DataRoomCompany> {
	return shownCompany(await caller.rpc('data_room_company', { target_company: companyID }), plane);
}

export async function companyShownToCircle(
	caller: SupabaseClient,
	companyID: string,
	circleID: string,
	plane: ControlPlaneCredentials
): Promise<DataRoomCompany> {
	return shownCompany(await caller.rpc('data_room_circle_company', { target_company: companyID, target_circle: circleID }), plane);
}

async function shownCompany({ data, error: refusal }: Answer, plane: ControlPlaneCredentials): Promise<DataRoomCompany> {
	if (refusal) error(502, 'the company could not be read');
	const { activity, ...company } = storedCompanySchema.parse(data);
	if (!activity) return company;
	return { ...company, activity: await withThumbnails(activity, controlPlane(plane)) };
}

async function withThumbnails(activity: StoredActivity, service: SupabaseClient): Promise<CompanyActivity> {
	const people = await Promise.all(
		Object.entries(activity.people).map(async ([key, person]) => {
			const image = person.picture ? await thumbnailOf(service, person.picture) : undefined;
			return [key, image ? { name: person.name, image } : { name: person.name }] as const;
		})
	);
	return { ...activity, people: Object.fromEntries(people) };
}

async function thumbnailOf(service: SupabaseClient, picturePath: string): Promise<string | undefined> {
	const { data } = await service.storage.from('asset').download(picturePath, {
		transform: { width: thumbnailSide, height: thumbnailSide, resize: 'cover' }
	});
	if (!data || data.size > largestThumbnailBytes || !data.type.startsWith('image/')) return undefined;
	const bytes = new Uint8Array(await data.arrayBuffer());
	return `data:${data.type};base64,${btoa(String.fromCharCode(...bytes))}`;
}
