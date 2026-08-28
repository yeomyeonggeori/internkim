import { currentAttendanceWorkPolicy } from './current-work-policy';
import { initialWorkPolicyEffectiveDate } from './work-policy-defaults';
import type { AttendanceWorkPolicyRevision } from './work-calendar-derivation';

export function storedWorkPolicyRevisions(
	value: unknown,
	whose: string
): AttendanceWorkPolicyRevision[] {
	if (typeof value !== 'object' || value === null) {
		throw new Error(`attendance work policy is invalid for ${whose}`);
	}
	const offered = value as Record<string, unknown>;
	if (!('revisions' in offered)) {
		return [
			{ ...currentAttendanceWorkPolicy(offered), effectiveDate: initialWorkPolicyEffectiveDate }
		];
	}
	if (!Array.isArray(offered.revisions) || offered.revisions.length === 0) {
		throw new Error(`attendance work policy revisions are invalid for ${whose}`);
	}
	const revisions = offered.revisions.map((revision) => {
		if (typeof revision !== 'object' || revision === null || !('effectiveDate' in revision)) {
			throw new Error(`attendance work policy revision is invalid for ${whose}`);
		}
		const effectiveDate = (revision as Record<string, unknown>).effectiveDate;
		if (typeof effectiveDate !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(effectiveDate)) {
			throw new Error(`attendance work policy revision date is invalid for ${whose}`);
		}
		return { ...currentAttendanceWorkPolicy(revision), effectiveDate };
	});
	return revisions.sort((left, right) => left.effectiveDate.localeCompare(right.effectiveDate));
}
