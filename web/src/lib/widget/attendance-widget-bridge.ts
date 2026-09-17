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

type AttendanceWidgetShell = { widget: AttendanceWidgetPlugin };

export async function attendanceWidgetShell(): Promise<AttendanceWidgetShell | null> {
	if (!isInsideNativeShell() || shellPlatform() !== 'ios') return null;
	const { registerPlugin } = await import('@capacitor/core');
	return { widget: registerPlugin<AttendanceWidgetPlugin>('AttendanceWidget') };
}
