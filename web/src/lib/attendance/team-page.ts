import { z } from 'zod';

const previewPersonSchema = z.strictObject({
	memberID: z.string().min(1),
	name: z.string(),
	email: z.string(),
	occurredAt: z.string().nullable()
});

const previewPeopleSchema = z.array(previewPersonSchema).max(4);

const teamSchema = z.strictObject({
	teamKey: z.string(),
	name: z.string(),
	isCallerTeam: z.boolean(),
	memberCount: z.number().int().nonnegative(),
	working: z.number().int().nonnegative(),
	done: z.number().int().nonnegative(),
	away: z.number().int().nonnegative(),
	needsCheckout: z.number().int().nonnegative(),
	notStarted: z.number().int().nonnegative(),
	preview: z.strictObject({
		working: previewPeopleSchema,
		done: previewPeopleSchema,
		away: previewPeopleSchema,
		notStarted: previewPeopleSchema
	}),
	recordedLocations: z.array(z.strictObject({
		name: z.string(), count: z.number().int().nonnegative()
	})),
	unknownLocationCount: z.number().int().nonnegative()
});

const memberSchema = z.strictObject({
	memberID: z.string().min(1),
	name: z.string(),
	email: z.string(),
	teamKey: z.string(),
	status: z.enum(['working', 'done', 'away', 'needs_checkout', 'not_started']),
	latestAt: z.string().nullable(),
	location: z.string().nullable()
});

export const attendanceTeamPageInputSchema = z.strictObject({
	pageKind: z.enum(['teams', 'members']),
	teamOffset: z.number().int().nonnegative().optional(),
	teamLimit: z.number().int().min(1).max(24).optional(),
	selectedTeamKey: z.string().min(1).optional(),
	memberOffset: z.number().int().nonnegative().optional(),
	memberLimit: z.number().int().min(1).max(48).optional(),
	searchText: z.string().max(100).optional(),
	locationFilter: z.string().max(100).optional(),
	statusFilter: z.enum(['working', 'done', 'away', 'not_started']).optional()
});

export const attendanceTeamPageSchema = z.strictObject({
	companyID: z.string().min(1),
	companyName: z.string().optional(),
	companySummary: z.strictObject({ memberCount: z.number().int().nonnegative(), working: z.number().int().nonnegative(), done: z.number().int().nonnegative(), away: z.number().int().nonnegative(), notStarted: z.number().int().nonnegative(), needsCheckout: z.number().int().nonnegative() }).optional(),
	timeZone: z.string(),
	serverTime: z.string(),
	authorization: z.strictObject({
		isAdmin: z.boolean(),
		teamViewVisibleToAll: z.boolean()
	}),
	teamOffset: z.number().int().nonnegative(),
	teamLimit: z.number().int().positive(),
	teamTotal: z.number().int().nonnegative(),
	teams: z.array(teamSchema),
	selectedTeamKey: z.string().nullable(),
	memberOffset: z.number().int().nonnegative(),
	memberLimit: z.number().int().positive(),
	memberTotal: z.number().int().nonnegative(),
	members: z.array(memberSchema)
});

export type AttendanceTeamPage = z.infer<typeof attendanceTeamPageSchema>;
export type AttendanceTeamPageInput = z.infer<typeof attendanceTeamPageInputSchema>;
