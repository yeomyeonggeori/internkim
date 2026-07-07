import { toast } from 'svelte-sonner';

export const calendarDeleteUndoTimeoutMs = 5000;
const calendarDeleteUndoToastID = 'calendar-delete-undo';

type CalendarDeleteUndoToastOptions = {
	message: string;
	actionLabel: string;
	undo: () => void;
};

export function showCalendarDeleteUndoToast(options: CalendarDeleteUndoToastOptions): void {
	toast(options.message, {
		id: calendarDeleteUndoToastID,
		duration: calendarDeleteUndoTimeoutMs,
		dismissible: false,
		class: 'calendar-delete-undo-toast w-auto min-w-0 max-w-[calc(100vw-32px)] rounded-md border-slate-200/20 bg-slate-950/95 px-3 py-2 text-slate-50 shadow-xl backdrop-blur',
		classes: {
			title: 'truncate text-[13px] font-semibold leading-none',
			actionButton: 'h-8 rounded-md px-2.5 text-[13px] font-bold'
		},
		action: {
			label: options.actionLabel,
			onClick: options.undo
		}
	});
}

export function dismissCalendarDeleteUndoToast(): void {
	toast.dismiss(calendarDeleteUndoToastID);
}
