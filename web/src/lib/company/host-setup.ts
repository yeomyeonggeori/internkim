import { z } from 'zod';

export const companyComputerName = 'company-computer';

export const hostInstallCommands = [
	'curl -fsSL https://intern.kim/install.sh | sh -s -- host',
	'sudo internkim install ~/Downloads/internkim-host.json'
].join('\n');

export const hostSetupRequestSchema = z.object({
	replaceExisting: z.boolean().default(false)
}).strict();

export const hostConfigurationSchema = z.object({
	schemaVersion: z.literal(1),
	appURL: z.string().url(),
	company: z.object({ id: z.guid(), name: z.string(), slug: z.string() }).strict(),
	centralPlane: z.object({ projectURL: z.string().url(), publishableKey: z.string().min(1) }).strict(),
	gatewayURL: z.string().url(),
	agentKey: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export type HostConfiguration = z.infer<typeof hostConfigurationSchema>;

export const hostSetupStatusSchema = z.object({
	company: hostConfigurationSchema.shape.company,
	hasConfiguration: z.boolean(),
	lastSeenAt: z.string().nullable()
}).strict();

export type HostSetupStatus = z.infer<typeof hostSetupStatusSchema>;
