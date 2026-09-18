import { attendanceActivityShell } from './attendance-activity-plugin';

export async function lockScreenRefusesTheClock(): Promise<boolean> {
	const shell = await attendanceActivityShell();
	if (!shell) return false;
	const { allowed } = await shell.activity.lockScreenState();
	return !allowed;
}
