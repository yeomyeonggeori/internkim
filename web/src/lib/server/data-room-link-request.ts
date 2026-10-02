import { error } from '@sveltejs/kit';
import { z } from 'zod';
import { asMember, controlPlane, planeCredentialsOf, type PlaneCredentials } from './control-plane';
import { verifiedRecordToken, recordTokenFor } from './record-token';
import type { Environment } from './agent-request';

const sessionSchema = z.object({
	sessionID: z.string().uuid(),
	companyID: z.string().uuid(),
	expiresAt: z.string()
});
export const dataRoomCookieName = 'data-room-session';

export function dataRoomPlane(environment: Environment): PlaneCredentials {
	const plane = planeCredentialsOf(environment);
	if (!plane) error(503, 'the data room is not configured');
	return plane;
}

export async function unlockDataRoomLink(
	plane: PlaneCredentials,
	linkID: string,
	accessCode: string,
	noticeVersion: string,
	clientAddress: string
) {
	const fingerprintBytes = await crypto.subtle.digest(
		'SHA-256',
		new TextEncoder().encode(`${linkID}\u0000${clientAddress}`)
	);
	const fingerprint = Array.from(new Uint8Array(fingerprintBytes), (byte) =>
		byte.toString(16).padStart(2, '0')
	).join('');
	const { data, error: refusal } = await controlPlane(plane).rpc('data_room_link_unlock', {
		target_link: linkID,
		access_code: accessCode,
		notice_version: noticeVersion,
		client_fingerprint: fingerprint
	});
	if (refusal) error(502, 'the data room could not be opened');
	const session = sessionSchema.safeParse(data);
	if (!session.success) error(403, 'the code or link is invalid, expired, or temporarily locked');
	const token = await recordTokenFor(plane.signingKey, plane.projectURL, {
		userID: session.data.sessionID,
		email: null,
		appMetadata: { dataRoomLinkID: linkID, dataRoomCompanyID: session.data.companyID }
	});
	return {
		...token,
		expiresAt: Math.min(token.expiresAt, Date.parse(session.data.expiresAt) / 1000)
	};
}

export async function dataRoomLinkCaller(
	plane: PlaneCredentials,
	linkID: string,
	accessToken: string | undefined
) {
	if (!accessToken) error(401, 'enter the code and acknowledge the notice');
	const claims = await verifiedRecordToken(plane.signingKey, plane.projectURL, accessToken);
	const metadata = z
		.object({ dataRoomLinkID: z.literal(linkID), dataRoomCompanyID: z.string().uuid() })
		.safeParse(claims?.app_metadata);
	if (!claims?.sub || !metadata.success) error(401, 'enter the code and acknowledge the notice');
	const caller = asMember(plane, accessToken);
	const { data, error: refusal } = await controlPlane(plane)
		.from('data_room_link_session')
		.select('expires_at,data_room_link!inner(id,company_id,expires_at,revoked_at,can_download)')
		.eq('id', claims.sub)
		.eq('link_id', linkID)
		.maybeSingle();
	if (refusal) error(502, 'the data room session could not be checked');
	const session = z
		.object({
			expires_at: z.string(),
			data_room_link: z.object({
				id: z.string(),
				company_id: z.string(),
				expires_at: z.string(),
				revoked_at: z.string().nullable(),
				can_download: z.boolean()
			})
		})
		.safeParse(data);
	if (
		!session.success ||
		session.data.data_room_link.revoked_at ||
		Date.parse(session.data.expires_at) <= Date.now() ||
		Date.parse(session.data.data_room_link.expires_at) <= Date.now()
	)
		error(403, 'this share link is no longer active');
	return {
		caller,
		companyID: metadata.data.dataRoomCompanyID,
		expiresAt: Date.parse(session.data.expires_at),
		canDownload: session.data.data_room_link.can_download
	};
}
