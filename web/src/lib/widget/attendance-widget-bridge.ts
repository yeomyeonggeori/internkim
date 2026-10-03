import { isInsideNativeShell, shellPlatform } from '$lib/native-shell/shell';

export type WidgetInstall = {
	installID: string;
	tokenName: string;
};

export type WidgetSupply = {
	token: string;
	tokenName: string;
	origin: string;
};

type AttendanceWidgetPlugin = {
	install(): Promise<WidgetInstall>;
	supply(supply: WidgetSupply): Promise<void>;
	forget(): Promise<void>;
};

export type WidgetPlatform = 'ios' | 'android';

const widgetPlatforms: readonly WidgetPlatform[] = ['ios', 'android'];

type AttendanceWidgetShell = { platform: WidgetPlatform; widget: AttendanceWidgetPlugin };

export async function attendanceWidgetShell(): Promise<AttendanceWidgetShell | null> {
	if (!isInsideNativeShell()) return null;
	const platform = widgetPlatformOf(shellPlatform());
	if (!platform) return null;
	const { registerPlugin } = await import('@capacitor/core');
	return { platform, widget: registerPlugin<AttendanceWidgetPlugin>('AttendanceWidget') };
}

function widgetPlatformOf(shell: string): WidgetPlatform | null {
	return widgetPlatforms.find((platform) => platform === shell) ?? null;
}
