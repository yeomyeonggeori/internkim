import type { AttendanceWidgetPlugin, WidgetInstall, WidgetSupply } from './attendance-widget-bridge';

const pluginName = 'attendance-widget';

export async function tauriAttendanceWidget(): Promise<AttendanceWidgetPlugin> {
	const { invoke } = await import('@tauri-apps/api/core');
	return {
		install: () => invoke<WidgetInstall>(`plugin:${pluginName}|install`),
		supply: (supply: WidgetSupply) => invoke<void>(`plugin:${pluginName}|supply`, { ...supply }),
		forget: () => invoke<void>(`plugin:${pluginName}|forget`)
	};
}
