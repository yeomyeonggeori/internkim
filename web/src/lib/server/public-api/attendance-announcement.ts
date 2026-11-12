import type { Environment } from '$lib/server/agent-request';
import { askTheProject } from '$lib/server/project-function';
import type { AttendanceAnnouncement, AttendanceAnnouncer } from './record/attendance-announcement';

type BackgroundWork = (work: Promise<unknown>) => void;

export function attendanceAnnouncedAs(
	environment: Environment,
	accessToken: string,
	backgroundWork?: BackgroundWork
): AttendanceAnnouncer {
	return async (announcement) => {
		const announcing = announce(environment, accessToken, announcement);
		if (backgroundWork) backgroundWork(announcing);
		else await announcing;
	};
}

async function announce(environment: Environment, accessToken: string, announcement: AttendanceAnnouncement): Promise<void> {
	try {
		const answer = await askTheProject(environment, 'announce-attendance', announcement, accessToken);
		if (answer.status >= 300) console.error('attendance.announcement_failed', { ...announcement, status: answer.status, body: answer.body });
	} catch (failure) {
		console.error('attendance.announcement_failed', { ...announcement, failure });
	}
}
