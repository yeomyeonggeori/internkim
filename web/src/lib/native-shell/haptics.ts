import type { HapticsPlugin } from '@capacitor/haptics';
import { isInsideNativeShell } from './shell';

export type HapticKind = 'selection' | 'touch' | 'success' | 'failure';

export function feelHaptic(kind: HapticKind): void {
	if (!isInsideNativeShell()) return;
	void playHaptic(kind);
}

async function playHaptic(kind: HapticKind): Promise<void> {
	const { Capacitor } = await import('@capacitor/core');
	if (!Capacitor.isPluginAvailable('Haptics')) return;
	const { Haptics, ImpactStyle, NotificationType } = await import('@capacitor/haptics');
	if (kind === 'selection') return playSelection(Haptics);
	if (kind === 'touch') return Haptics.impact({ style: ImpactStyle.Light });
	if (kind === 'success') return Haptics.notification({ type: NotificationType.Success });
	return Haptics.notification({ type: NotificationType.Error });
}

async function playSelection(haptics: HapticsPlugin): Promise<void> {
	await haptics.selectionStart();
	await haptics.selectionChanged();
	await haptics.selectionEnd();
}
