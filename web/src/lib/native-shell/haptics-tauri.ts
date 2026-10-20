import type { HapticKind } from './haptics';

export async function playTauriHaptic(kind: HapticKind): Promise<void> {
	const { impactFeedback, notificationFeedback, selectionFeedback } = await import('@tauri-apps/plugin-haptics');
	if (kind === 'selection') await selectionFeedback();
	else if (kind === 'touch') await impactFeedback('light');
	else if (kind === 'success') await notificationFeedback('success');
	else await notificationFeedback('error');
}
