type KeyboardChange = { isOpen: boolean };

export async function listenToTauriKeyboard(onChange: (isOpen: boolean) => void): Promise<() => void> {
	const { addPluginListener } = await import('@tauri-apps/api/core');
	const listener = await addPluginListener<KeyboardChange>('keyboard', 'change', (change) => onChange(change.isOpen));
	return () => void listener.unregister();
}
