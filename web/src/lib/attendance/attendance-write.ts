export type AttendanceWriteOutcome = 'blocked' | 'saved' | 'backdated';

export type AttendanceWriteAuthority = {
	isAdmin: boolean;
	currentUserEmail: string;
	backdatedAfterMinutes?: number;
};

export type AttendanceWriteSubject = {
	subjectEmail: string;
	anchorTime: Date;
};

export type AttendanceWriteResult = { outcome: 'saved' | 'backdated' };

const writtenStatuses = ['added', 'corrected', 'removed'];

export function attendanceWriteOutcome(
	authority: AttendanceWriteAuthority,
	subject: AttendanceWriteSubject,
	currentTime: Date
): AttendanceWriteOutcome {
	if (authority.backdatedAfterMinutes === undefined) return 'saved';
	if (authority.isAdmin) return 'saved';
	if (subject.subjectEmail !== authority.currentUserEmail) return 'blocked';
	if (!Number.isFinite(currentTime.getTime()) || !Number.isFinite(subject.anchorTime.getTime())) {
		return 'blocked';
	}
	return isBackdated(authority.backdatedAfterMinutes, subject.anchorTime, currentTime)
		? 'backdated'
		: 'saved';
}

export function combineAttendanceWriteOutcomes(
	outcomes: AttendanceWriteOutcome[]
): AttendanceWriteOutcome {
	if (outcomes.length === 0) return 'blocked';
	if (outcomes.includes('blocked')) return 'blocked';
	return outcomes.includes('backdated') ? 'backdated' : 'saved';
}

export function attendanceWriteResultFrom(data: unknown): AttendanceWriteResult {
	if (!data || typeof data !== 'object' || Array.isArray(data)) {
		throw new Error('the attendance write answered without a status');
	}
	const answered = data as Record<string, unknown>;
	const status = answered.status;
	if (typeof status !== 'string') throw new Error('the attendance write answered without a status');
	if (!writtenStatuses.includes(status)) {
		throw new Error(`the attendance write answered an unknown status ${status}`);
	}
	return { outcome: answered.backdated === true ? 'backdated' : 'saved' };
}

function isBackdated(
	backdatedAfterMinutes: number,
	anchorTime: Date,
	currentTime: Date
): boolean {
	return anchorTime.getTime() < currentTime.getTime() - backdatedAfterMinutes * 60_000;
}
