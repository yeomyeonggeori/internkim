import { z } from 'zod';
import { hostConfigurationSchema } from './host-setup';

export const boxKeySchema = z.string().regex(/^[A-Za-z0-9_-]{43}$/);

export const boxHostNameSchema = z.string().regex(/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/);

export const pairingPageAddressSchema = z
	.string()
	.regex(
		/^http:\/\/([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.local|10(\.\d{1,3}){3}|172\.(1[6-9]|2\d|3[01])(\.\d{1,3}){2}|192\.168(\.\d{1,3}){2}):\d{1,5}\/$/
	);

export const boxAnnouncementSchema = z.object({
	encryptionKey: boxKeySchema,
	wantsPairingCode: z.boolean().optional(),
	hostName: boxHostNameSchema.optional(),
	pairingPageAddresses: z.array(pairingPageAddressSchema).max(2).optional()
}).strict();

export const emptyBoxSchema = z.object({
	publicKey: boxKeySchema,
	announcedAt: z.string(),
	hostName: boxHostNameSchema.nullable(),
	pairingPageAddresses: z.array(pairingPageAddressSchema)
}).strict();

export type EmptyBox = z.infer<typeof emptyBoxSchema>;

export const boxClaimSchema = z.object({
	publicKey: boxKeySchema,
	pairingCode: z.string().trim().min(1).max(32)
}).strict();

export const boxFileClaimSchema = z.object({
	encryptionKey: boxKeySchema,
	connectionKey: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export const sealedSecretVersion = 1;

export const sealedSecretSchema = z.object({
	version: z.literal(sealedSecretVersion),
	recipient: boxKeySchema,
	enc: boxKeySchema,
	ciphertext: z.string().regex(/^[A-Za-z0-9_-]+$/).max(4096)
}).strict();

export type SealedSecret = z.infer<typeof sealedSecretSchema>;

export const boxConfigurationSchema = hostConfigurationSchema.omit({ agentKey: true });

export type BoxConfiguration = z.infer<typeof boxConfigurationSchema>;

export const connectedBoxSchema = z.object({
	companyID: z.string(),
	publicKey: boxKeySchema,
	encryptionKey: boxKeySchema,
	lastSeenAt: z.string().nullable(),
	hasModelKey: z.boolean()
}).strict();

export type ConnectedBox = z.infer<typeof connectedBoxSchema>;
