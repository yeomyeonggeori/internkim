import { isInsideTauriShell, shellPlatform } from '$lib/native-shell/shell';
import { tauriAttendanceWidget } from './attendance-widget-tauri';

export type WidgetInstall = {
	installID: string;
	tokenName: string;
};

export type WidgetSupply = {
	token: string;
	tokenName: string;
	origin: string;
};

export type AttendanceWidgetPlugin = {
	install(): Promise<WidgetInstall>;
	supply(supply: WidgetSupply): Promise<void>;
	forget(): Promise<void>;
	backgroundRefreshState(): Promise<{ allowed: boolean }>;
	openSettings(): Promise<void>;
};

export type WidgetPlatform = 'ios' | 'android';

const widgetPlatforms: readonly WidgetPlatform[] = ['ios', 'android'];

type AttendanceWidgetShell = { platform: WidgetPlatform; widget: AttendanceWidgetPlugin };

export async function attendanceWidgetShell(): Promise<AttendanceWidgetShell | null> {
	const platform = widgetPlatformOf(shellPlatform());
	if (!platform) return null;
	if (!isInsideTauriShell()) return null;
	return { platform, widget: await tauriAttendanceWidget() };
}

function widgetPlatformOf(shell: string): WidgetPlatform | null {
	return widgetPlatforms.find((platform) => platform === shell) ?? null;
}
