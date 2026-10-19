import { boxFingerprintOf } from '$lib/company/box';
import type { Boxes } from './host-setup-client';

export type BoxStep = 'searching' | 'choosing' | 'givingModelKey' | 'connected';

export function boxStepOf(boxes: Boxes): BoxStep {
	if (boxes.boxes.length > 0 && boxes.hasModelKey) return 'connected';
	if (boxes.boxes.length > 0) return 'givingModelKey';
	if (boxes.empty.length > 0) return 'choosing';
	return 'searching';
}

export function shortBoxName(publicKey: string): string {
	return `…${boxFingerprintOf(publicKey)}`;
}
