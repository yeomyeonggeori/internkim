import { z } from 'zod';
import { hostConfigurationSchema } from './host-setup';

export const boxKeySchema = z.string().regex(/^[A-Za-z0-9_-]{43}$/);

export const boxAnnouncementSchema = z.object({ encryptionKey: boxKeySchema }).strict();

export const emptyBoxSchema = z.object({
	publicKey: boxKeySchema,
	announcedAt: z.string()
}).strict();

export type EmptyBox = z.infer<typeof emptyBoxSchema>;

export const boxClaimSchema = z.object({ publicKey: boxKeySchema }).strict();

export const boxFileClaimSchema = z.object({
	encryptionKey: boxKeySchema,
	connectionKey: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export const sealedModelKeySchema = z.object({
	ephemeralPublicKey: boxKeySchema,
	nonce: z.string().regex(/^[A-Za-z0-9_-]{16}$/),
	ciphertext: z.string().regex(/^[A-Za-z0-9_-]+$/).max(4096)
}).strict();

export type SealedModelKey = z.infer<typeof sealedModelKeySchema>;

export const boxConfigurationSchema = hostConfigurationSchema.omit({ agentKey: true });

export type BoxConfiguration = z.infer<typeof boxConfigurationSchema>;

export const connectedBoxSchema = z.object({
	publicKey: boxKeySchema,
	encryptionKey: boxKeySchema,
	lastSeenAt: z.string().nullable(),
	hasModelKey: z.boolean()
}).strict();

export type ConnectedBox = z.infer<typeof connectedBoxSchema>;
