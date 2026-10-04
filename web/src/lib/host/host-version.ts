import { z } from 'zod';

export const releaseTagSchema = z.string().regex(/^v\d{4}\.\d{2}\.\d{2}\.\d{6}$/);

export const hostReleaseSchema = z.strictObject({
	version: z.string().min(1),
	publishedAt: z.string(),
	notes: z.string().optional()
});

export const hostVersionGetResultSchema = z.strictObject({
	installedVersion: z.string(),
	channel: z.enum(['stable', 'testing', 'unrecorded']),
	updateMethod: z.enum(['apt', 'dnf', 'pacman', 'brew', '']),
	latestStable: hostReleaseSchema.optional(),
	previousStable: hostReleaseSchema.optional(),
	isUpdateAvailable: z.boolean(),
	expectedDowntimeSeconds: z.int().min(1).optional(),
	updateInProgress: z
		.strictObject({
			fromVersion: z.string(),
			toVersion: z.string(),
			startedAt: z.string()
		})
		.optional(),
	lastUpdate: z
		.strictObject({
			fromVersion: z.string(),
			toVersion: z.string(),
			startedAt: z.string(),
			finishedAt: z.string(),
			succeeded: z.boolean(),
			error: z.string().optional()
		})
		.optional()
});

export const hostUpdateResultSchema = z.strictObject({
	status: z.literal('started'),
	fromVersion: z.string(),
	toVersion: z.string(),
	startedAt: z.string(),
	expectedDowntimeSeconds: z.int().min(1)
});

export type HostRelease = z.infer<typeof hostReleaseSchema>;
export type HostVersion = z.infer<typeof hostVersionGetResultSchema>;
export type HostUpdateStarted = z.infer<typeof hostUpdateResultSchema>;
