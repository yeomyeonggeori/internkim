import { z } from 'zod';

export const taskInvitationSchema = z.strictObject({
	invitationID: z.string().uuid(),
	email: z.string(),
	businesses: z.array(z.string()),
	createdAt: z.string(),
	acceptedAt: z.string().nullable()
});

export const taskInvitationListResultSchema = z.strictObject({ invitations: z.array(taskInvitationSchema) });
