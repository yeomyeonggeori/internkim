import { playTauriHaptic } from './haptics-tauri';
import { isInsideTauriPhoneShell } from './shell';

export type HapticKind = 'selection' | 'touch' | 'success' | 'failure';

export function feelHaptic(kind: HapticKind): void {
	if (isInsideTauriPhoneShell()) void playTauriHaptic(kind);
}
