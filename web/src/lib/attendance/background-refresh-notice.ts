import { toast } from 'svelte-sonner';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { attendanceWidgetShell } from '$lib/widget/attendance-widget-bridge';
import { attendanceText } from '../../routes/attendance/text';

const text = createPageText(attendanceText);
const shownKey = 'internkim.widget.background-refresh-off.shown';
const noticeID = 'widget-background-refresh-off';
let watching = false;

export function tellWhenTheWidgetCannotFollow(): void {
	if (!watching) {
		watching = true;
		document.addEventListener('visibilitychange', () => {
			if (document.visibilityState === 'visible') void lookAtBackgroundRefresh();
		});
	}
	void lookAtBackgroundRefresh();
}

async function lookAtBackgroundRefresh(): Promise<void> {
	const shell = await attendanceWidgetShell();
	if (shell?.platform !== 'ios') return;
	const { allowed } = await shell.widget.backgroundRefreshState();
	if (allowed) {
		forgetShowing();
		toast.dismiss(noticeID);
		return;
	}
	if (wasShown()) return;
	rememberShowing();
	toast(text.widgetRefreshOff, {
		id: noticeID,
		description: text.widgetRefreshOffDetail,
		duration: Number.POSITIVE_INFINITY,
		position: 'bottom-center',
		classes: {
			toast: 'flex-wrap gap-y-3',
			content: 'basis-full',
			cancelButton: 'order-1 ml-1.5!'
		},
		action: { label: text.openSettings, onClick: () => void shell.widget.openSettings() },
		cancel: { label: text.close, onClick: () => undefined }
	});
}

function wasShown(): boolean {
	try {
		return window.localStorage.getItem(shownKey) === 'yes';
	} catch {
		return false;
	}
}

function rememberShowing(): void {
	try {
		window.localStorage.setItem(shownKey, 'yes');
	} catch {
		return;
	}
}

function forgetShowing(): void {
	try {
		window.localStorage.removeItem(shownKey);
	} catch {
		return;
	}
}
