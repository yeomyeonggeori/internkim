type CalendarThemeApp = {
	updateConfig: (configuration: { theme: { mode: 'dark' | 'light' } }) => void;
};

export function syncCalendarThemeToDocument(calendarApp: CalendarThemeApp): void {
	const mode = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
	calendarApp.updateConfig({ theme: { mode } });
}
