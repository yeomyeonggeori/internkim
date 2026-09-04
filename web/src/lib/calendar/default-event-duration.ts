export const defaultEventDurationMinutes = 60;

export function defaultEventEndDate(startDate: Date): Date {
	return new Date(startDate.getTime() + defaultEventDurationMinutes * 60000);
}
