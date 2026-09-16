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

export async function attendanceWidgetShell(): Promise<AttendanceWidgetPlugin | null> {
	if (!isInsideNativeShell() || shellPlatform() !== 'ios') return null;
	const { registerPlugin } = await import('@capacitor/core');
	return registerPlugin<AttendanceWidgetPlugin>('AttendanceWidget');
}
