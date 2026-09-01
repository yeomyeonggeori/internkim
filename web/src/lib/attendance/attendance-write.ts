export type AttendanceWriteOutcome = 'blocked' | 'saved' | 'requested';

export type AttendanceWriteAuthority = {
	isAdmin: boolean;
	currentUserEmail: string;
	correctionWindowMinutes?: number;
};

export type AttendanceWriteSubject = {
	subjectEmail: string;
	anchorTime: Date;
};

export type AttendanceWriteResult =
	| { outcome: 'saved' }
	| { outcome: 'requested'; approvalID: string };

const savedStatuses = ['added', 'corrected', 'removed'];

export function attendanceWriteOutcome(
	authority: AttendanceWriteAuthority,
	subject: AttendanceWriteSubject,
	currentTime: Date
): AttendanceWriteOutcome {
	if (authority.correctionWindowMinutes === undefined) return 'saved';
	if (authority.isAdmin) return 'saved';
	if (subject.subjectEmail !== authority.currentUserEmail) return 'blocked';
	if (!Number.isFinite(currentTime.getTime()) || !Number.isFinite(subject.anchorTime.getTime())) {
		return 'blocked';
	}
	return isWithinCorrectionWindow(authority.correctionWindowMinutes, subject.anchorTime, currentTime)
		? 'saved'
		: 'requested';
}

export function combineAttendanceWriteOutcomes(
	outcomes: AttendanceWriteOutcome[]
): AttendanceWriteOutcome {
	if (outcomes.length === 0) return 'blocked';
	if (outcomes.includes('blocked')) return 'blocked';
	return outcomes.includes('requested') ? 'requested' : 'saved';
}

export function attendanceWriteResultFrom(data: unknown): AttendanceWriteResult {
	if (!data || typeof data !== 'object' || Array.isArray(data)) {
		throw new Error('the attendance write answered without a status');
	}
	const answered = data as Record<string, unknown>;
	const status = answered.status;
	if (typeof status !== 'string') throw new Error('the attendance write answered without a status');
	if (savedStatuses.includes(status)) return { outcome: 'saved' };
	if (status === 'approval_requested') {
		const approvalID = answered.approvalID;
		if (typeof approvalID !== 'string' || approvalID === '') {
			throw new Error('the attendance write opened a request without naming it');
		}
		return { outcome: 'requested', approvalID };
	}
	throw new Error(`the attendance write answered an unknown status ${status}`);
}

function isWithinCorrectionWindow(
	correctionWindowMinutes: number,
	anchorTime: Date,
	currentTime: Date
): boolean {
	const windowOpensAt = currentTime.getTime() - correctionWindowMinutes * 60_000;
	return anchorTime.getTime() >= windowOpensAt;
}
