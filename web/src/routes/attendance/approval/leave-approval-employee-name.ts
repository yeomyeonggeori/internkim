import type { AttendanceMember } from '../attendance-context.svelte';

export function leaveApprovalEmployeeName(
	members: AttendanceMember[],
	employeeEmail: string
): string {
	const member = members.find(
		(candidate) => candidate.email.toLocaleLowerCase() === employeeEmail.toLocaleLowerCase()
	);
	return member?.displayName || employeeEmail;
}
