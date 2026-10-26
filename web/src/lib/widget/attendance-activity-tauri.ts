import type { ActivityToken, AttendanceActivityPlugin } from './attendance-activity-plugin';

const pluginName = 'attendance-activity';

export async function tauriAttendanceActivity(): Promise<AttendanceActivityPlugin> {
	const { addPluginListener, invoke } = await import('@tauri-apps/api/core');
	return {
		watchTokens: () => invoke<void>(`plugin:${pluginName}|watch_tokens`),
		heldTokens: () => invoke<{ tokens: ActivityToken[] }>(`plugin:${pluginName}|held_tokens`),
		lockScreenState: () => invoke<{ allowed: boolean }>(`plugin:${pluginName}|lock_screen_state`),
		closeCards: () => invoke<void>(`plugin:${pluginName}|close_cards`),
		addListener: async (event, listener) => {
			const registered = await addPluginListener<ActivityToken>(pluginName, event, listener);
			return { remove: () => registered.unregister() };
		}
	};
}
