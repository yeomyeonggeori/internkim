import { toast } from 'svelte-sonner';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { attendanceActivityShell } from '$lib/widget/attendance-activity-plugin';
import { lockScreenRefusesTheClock } from '$lib/widget/attendance-lock-screen';
import { attendanceText } from '../../routes/attendance/text';
import { myAttendanceToday } from './my-attendance-today.svelte';

const text = createPageText(attendanceText);
let reminded = false;
let watching = false;

export function catchTheLockScreenUp(): void {
	if (!watching) {
		watching = true;
		document.addEventListener('visibilitychange', () => {
			if (document.visibilityState === 'visible') void catchUp();
		});
	}
	void catchUp();
}

async function catchUp(): Promise<void> {
	const shell = await attendanceActivityShell();
	if (!shell) return;
	if (!(await myAttendanceToday.load())) return;
	if (myAttendanceToday.status !== 'working') {
		await shell.activity.closeCards();
		return;
	}
	if (reminded || !(await lockScreenRefusesTheClock())) return;
	reminded = true;
	toast.info(text.lockScreenOff);
}
