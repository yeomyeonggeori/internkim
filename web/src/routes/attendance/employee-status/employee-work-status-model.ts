import type { AttendanceEmployeeWorkStatus } from '../attendance-api';

export function filterEmployeeWorkStatuses(
	employees: AttendanceEmployeeWorkStatus[],
	query: string,
	status: string
): AttendanceEmployeeWorkStatus[] {
	const normalizedQuery = query.trim().toLocaleLowerCase();
	return employees.filter((employee) => {
		const matchesQuery =
			!normalizedQuery ||
			employee.displayName.toLocaleLowerCase().includes(normalizedQuery) ||
			employee.email.toLocaleLowerCase().includes(normalizedQuery);
		const matchesStatus = employeeMatchesWorkStatus(employee, status);
		return matchesQuery && matchesStatus;
	});
}

function employeeMatchesWorkStatus(employee: AttendanceEmployeeWorkStatus, status: string): boolean {
	switch (status) {
		case 'all':
			return true;
		case 'overtime':
			return employee.overtimeMinutes > 0;
		case 'remaining':
			return employee.remainingMinutes > 0;
		case 'coreTimeMissed':
			return employee.coreTimeMissed;
		case 'lateOrEarly':
			return employee.late || employee.earlyLeave;
		case 'needsReview':
			return employee.needsReview;
		default:
			return employee.status === status;
	}
}
