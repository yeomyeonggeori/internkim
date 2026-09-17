import { withQueuedInput, type HandoffInput } from './handoff-input';

const largestInputsPerCall = 200;

export class HandoffInputQueue {
	private queued: HandoffInput[] = [];
	private isSending = false;

	constructor(
		private readonly send: (inputs: HandoffInput[]) => Promise<void>,
		private readonly report: (failure: unknown) => void
	) {}

	push(input: HandoffInput): void {
		this.queued = withQueuedInput(this.queued, input);
		void this.sendWhatWaits();
	}

	private async sendWhatWaits(): Promise<void> {
		if (this.isSending || this.queued.length === 0) return;
		const batch = this.queued.slice(0, largestInputsPerCall);
		this.queued = this.queued.slice(batch.length);
		this.isSending = true;
		await this.send(batch).catch(this.report);
		this.isSending = false;
		void this.sendWhatWaits();
	}
}
