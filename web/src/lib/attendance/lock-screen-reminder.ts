import { toast } from 'svelte-sonner';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { lockScreenRefusesTheClock } from '$lib/widget/attendance-lock-screen';
import { attendanceText } from '../../routes/attendance/text';
import { myAttendanceToday } from './my-attendance-today.svelte';

const text = createPageText(attendanceText);
let reminded = false;
let watching = false;

export function remindWhereToTurnTheLockScreenOn(): void {
	if (!watching) {
		watching = true;
		document.addEventListener('visibilitychange', () => {
			if (document.visibilityState === 'visible') void remind();
		});
	}
	void remind();
}

async function remind(): Promise<void> {
	if (reminded) return;
	if (!(await lockScreenRefusesTheClock())) return;
	await myAttendanceToday.load();
	if (myAttendanceToday.status !== 'working') return;
	reminded = true;
	toast.info(text.lockScreenOff);
}
