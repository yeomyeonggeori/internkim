import { z } from 'zod';

export const savedAttendanceEventSchema = z.strictObject({
	id: z.string(),
	personID: z.string(),
	kind: z.enum(['clock_in', 'clock_out']),
	occurredAt: z.string(),
	location: z.string().nullable()
});

export type SavedAttendanceEvent = z.infer<typeof savedAttendanceEventSchema>;
