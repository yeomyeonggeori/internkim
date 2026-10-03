import { z } from 'zod';

export const dataRoomLinkLifetimeHours = [6, 12, 24, 48, 72, 168] as const;
export const dataRoomNoticeVersion = '1';
export const dataRoomLinkSessionSchema = z.object({
	sessionID: z.string().uuid(),
	companyID: z.guid(),
	expiresAt: z.iso.datetime({ offset: true })
});
export const dataRoomAccessCodePattern = '[0-9]{6}';
const dataRoomAccessCodeSchema = z.string().regex(new RegExp(`^${dataRoomAccessCodePattern}$`));
export const dataRoomLinkInputSchema = z.strictObject({
	circleID: z.string().min(1),
	label: z.string().trim().min(1).max(120),
	lifetimeHours: z.literal(dataRoomLinkLifetimeHours).default(72),
	canDownload: z.boolean().default(false)
});
export const dataRoomLinkSchema = z.strictObject({
	id: z.string().uuid(),
	circleID: z.string(),
	label: z.string(),
	canDownload: z.boolean(),
	createdAt: z.string(),
	expiresAt: z.string(),
	revokedAt: z.string().nullable()
});
export const dataRoomLinksSchema = z.strictObject({
	links: z.array(dataRoomLinkSchema), shareableCircleIDs: z.array(z.string()), downloadableCircleIDs: z.array(z.string())
});
export const dataRoomLinkCreatedSchema = z.strictObject({
	linkID: z.string().uuid(),
	accessCode: dataRoomAccessCodeSchema
});
export const dataRoomLinkRevokeSchema = z.strictObject({ linkID: z.string().uuid() });
export const dataRoomUnlockSchema = z.strictObject({
	accessCode: dataRoomAccessCodeSchema,
	noticeVersion: z.literal(dataRoomNoticeVersion)
});

export function generateDataRoomCode(): string {
	const randomValues = new Uint32Array(1);
	do {
		crypto.getRandomValues(randomValues);
	} while (randomValues[0] >= 4294000000);
	return String(randomValues[0] % 1000000).padStart(6, '0');
}
