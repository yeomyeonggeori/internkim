import type { PushOutcome } from './push-vocabulary.ts';

export type PushChannel = 'apns' | 'fcm' | 'web-push';

export type PushStage = 'authorization' | 'subscription' | 'encryption' | 'request' | 'answer';

export type PushNonDelivery = {
	channel: PushChannel;
	address: string;
	stage: PushStage;
	outcome: Exclude<PushOutcome, 'delivered'>;
	status?: number;
	reason?: string;
	failure?: unknown;
};

export function shortAddress(address: string): string {
	if (address === '') return '(empty)';
	try {
		return new URL(address).host;
	} catch {
		return `${address.slice(0, 8)}… (${address.length} chars)`;
	}
}

function saidAbout(failure: unknown): string {
	if (failure === undefined || failure === null) return '';
	if (failure instanceof Error) return `${failure.name}: ${failure.message}`;
	return String(failure);
}

export function sayPushNotDelivered(nonDelivery: PushNonDelivery): void {
	const said: Record<string, unknown> = {
		event: 'push.not-delivered',
		channel: nonDelivery.channel,
		address: shortAddress(nonDelivery.address),
		stage: nonDelivery.stage,
		outcome: nonDelivery.outcome
	};
	if (nonDelivery.status !== undefined) said.status = nonDelivery.status;
	if (nonDelivery.reason) said.reason = nonDelivery.reason;
	const failure = saidAbout(nonDelivery.failure);
	if (failure) said.failure = failure;
	const write = nonDelivery.outcome === 'gone' ? console.warn : console.error;
	write(JSON.stringify(said));
}
