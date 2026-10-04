import { isInsideNativeShell } from './shell';

export type HapticKind = 'touch' | 'success' | 'failure';

export function feelHaptic(kind: HapticKind): void {
	if (!isInsideNativeShell()) return;
	void playHaptic(kind);
}

async function playHaptic(kind: HapticKind): Promise<void> {
	const { Capacitor } = await import('@capacitor/core');
	if (!Capacitor.isPluginAvailable('Haptics')) return;
	const { Haptics, ImpactStyle, NotificationType } = await import('@capacitor/haptics');
	if (kind === 'touch') return Haptics.impact({ style: ImpactStyle.Light });
	if (kind === 'success') return Haptics.notification({ type: NotificationType.Success });
	return Haptics.notification({ type: NotificationType.Error });
}
