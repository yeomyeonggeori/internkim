import type { Boxes } from './host-setup-client';

export type BoxStep = 'searching' | 'choosing' | 'givingModelKey' | 'connected';

export function boxStepOf(boxes: Boxes): BoxStep {
	if (boxes.connected?.hasModelKey) return 'connected';
	if (boxes.connected) return 'givingModelKey';
	if (boxes.empty.length > 0) return 'choosing';
	return 'searching';
}

export function shortBoxName(publicKey: string): string {
	return publicKey.slice(0, 8);
}
