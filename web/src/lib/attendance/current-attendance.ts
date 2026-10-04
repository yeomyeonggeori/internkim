import { z } from 'zod';
import { savedAttendanceEventSchema } from './recorded-attendance';

export const currentAttendanceSchema = z.strictObject({
	memberID: z.string(),
	email: z.string(),
	companyID: z.string(),
	timeZone: z.string(),
	serverTime: z.string(),
	backdatedAfterMinutes: z.number().int().positive(),
	workLocations: z.array(z.strictObject({ name: z.string(), color: z.string().nullable() })),
	authorization: z.strictObject({ isAdmin: z.boolean(), teamViewVisibleToAll: z.boolean() }),
	todayEvents: z.array(savedAttendanceEventSchema),
	latestEvent: savedAttendanceEventSchema.nullable(),
	activeLeave: z.strictObject({
		leaveID: z.string(), kindID: z.string(), kindName: z.string().nullable(),
		days: z.number(), startsAt: z.string(), endsAt: z.string()
	}).nullable()
});

export type CurrentAttendance = z.infer<typeof currentAttendanceSchema>;
