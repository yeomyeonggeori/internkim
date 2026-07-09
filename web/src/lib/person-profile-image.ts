export function personProfileImagePath(personID: string | undefined): string {
	const trimmedPersonID = personID?.trim() ?? '';
	if (!trimmedPersonID) return '';
	return `/calendar/api/participants/${encodeURIComponent(trimmedPersonID)}/image`;
}
