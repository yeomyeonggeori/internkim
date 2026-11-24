export async function clearReadNotificationsOnTheMac(readTagOf: (pushed: unknown) => string): Promise<() => void> {
	const { addPluginListener, invoke } = await import('@tauri-apps/api/core');
	const listener = await addPluginListener<unknown>('notifications', 'push-message', (pushed) => {
		const tag = readTagOf(pushed);
		if (!tag) return;
		invoke('plugin:foreground-notifications|clear_delivered', { tag }).catch((failure: unknown) =>
			console.warn('the notifications of a conversation read elsewhere were not cleared', failure)
		);
	});
	return () => void listener.unregister().catch(() => undefined);
}
