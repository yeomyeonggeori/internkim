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

export const boxVerificationSchema = z.object({
	publicKey: boxKeySchema,
	pairingCode: z.string().trim().min(1).max(32)
}).strict();

export const boxClaimSchema = z.object({
	publicKey: boxKeySchema,
	ticket: z.string().regex(/^[A-Za-z0-9_-]{43}$/)
}).strict();

export const verifiedBoxSchema = z.object({
	ticket: z.string(),
	publicKey: boxKeySchema,
	hostName: boxHostNameSchema.nullable(),
	publicAddress: z.string(),
	companyName: z.string()
}).strict();

export type VerifiedBoxAnswer = z.infer<typeof verifiedBoxSchema>;

export function boxFingerprintOf(publicKey: string): string {
	return publicKey.slice(-4);
}

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

export type WifiNetwork = { ssid: string; password: string };

export const wifiOutcomeResultSchema = z.enum(['joined', 'failed']);

export type WifiOutcomeResult = z.infer<typeof wifiOutcomeResultSchema>;

export const wifiOutcomeSchema = z.object({
	requestID: z.uuid(),
	result: wifiOutcomeResultSchema,
	reportedAt: z.iso.datetime({ offset: true })
}).strict();

export type WifiOutcome = z.infer<typeof wifiOutcomeSchema>;

export const maximumNearbyNetworks = 50;

export const maximumSSIDBytes = 32;

export const nearbyNetworkSchema = z.object({
	ssid: z.string().min(1).refine((ssid) => new TextEncoder().encode(ssid).length <= maximumSSIDBytes),
	signalPercent: z.number().int().min(0).max(100),
	isSecured: z.boolean(),
	isConnected: z.boolean().optional()
}).strict();

export type NearbyNetwork = z.infer<typeof nearbyNetworkSchema>;

export const nearbyNetworkListSchema = z
	.array(nearbyNetworkSchema)
	.max(maximumNearbyNetworks)
	.refine((networks) => new Set(networks.map((network) => network.ssid)).size === networks.length);

export const wifiChangeStatusSchema = z.object({
	pendingRequestID: z.uuid().nullable(),
	outcome: wifiOutcomeSchema.nullable(),
	nearbyNetworks: nearbyNetworkListSchema,
	scannedAt: z.iso.datetime({ offset: true }).nullable()
}).strict();

export type WifiChangeStatus = z.infer<typeof wifiChangeStatusSchema>;
