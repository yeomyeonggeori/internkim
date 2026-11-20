import { z } from 'zod';
import type { LinkPreview } from '$lib/messenger/messenger-api';
import { publicDataRoomRead } from './guest';

const websitePreviewAnswerSchema = z.object({
	preview: z
		.object({
			url: z.string(),
			title: z.string(),
			description: z.string(),
			siteName: z.string(),
			imageURL: z.string(),
			video: z.object({ url: z.string(), kind: z.enum(['file', 'player']), width: z.number(), height: z.number() }).nullable()
		})
		.nullable()
});

export async function publicWebsitePreviewOf(companyID: string): Promise<LinkPreview | null> {
	const answer = await publicDataRoomRead(`/api/v1/data-room/${companyID}/website-preview`);
	return websitePreviewAnswerSchema.parse(answer).preview;
}
