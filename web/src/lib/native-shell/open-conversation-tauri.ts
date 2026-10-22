export async function holdBackTauriNotifications(tag: string): Promise<void> {
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke('plugin:foreground-notifications|hold_back', { tag });
}
