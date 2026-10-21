import { ToolRefused } from '$lib/public-api-call';
import { readHostVersion, readHostWhileItMayBeAway, startHostUpdate } from './host-update-client';
import {
	hostActionsFor,
	isSettled,
	refusalMessageOf,
	stageOfTheUpdate,
	updateCheckMilliseconds,
	watchOfTheRunningUpdate,
	type UpdateStage,
	type UpdateWatch
} from './host-update-model';
import type { HostRelease, HostVersion } from './host-version';

export type HostUpdateConfirmation = { release: HostRelease; isGoingBack: boolean };

export type HostUpdateRefusalText = { refusals: Readonly<Record<string, string>>; startFailed: string };

class HostUpdate {
	version = $state.raw<HostVersion | null>(null);
	isReading = $state(false);
	confirmation = $state.raw<HostUpdateConfirmation | null>(null);
	isStarting = $state(false);
	refusal = $state('');
	watch = $state.raw<UpdateWatch | null>(null);
	stage = $state.raw<UpdateStage | null>(null);
	settled = $state.raw<{ watch: UpdateWatch; stage: UpdateStage } | null>(null);

	actions = $derived(this.version ? hostActionsFor(this.version) : null);
	availableRelease = $derived(this.actions?.kind === 'choices' ? this.actions.update : null);

	read = async () => {
		this.isReading = true;
		try {
			this.version = await readHostVersion();
			const running = watchOfTheRunningUpdate(this.version);
			if (running && !this.watch) void this.follow(running);
		} catch (unanswered) {
			if (!(unanswered instanceof ToolRefused)) throw unanswered;
			this.version = null;
		} finally {
			this.isReading = false;
		}
	};

	confirm = (release: HostRelease, isGoingBack: boolean) => {
		this.refusal = '';
		this.confirmation = { release, isGoingBack };
	};

	dismiss = () => {
		if (!this.isStarting) this.confirmation = null;
	};

	start = async (text: HostUpdateRefusalText) => {
		if (!this.confirmation) return;
		const target = this.confirmation.release.version;
		this.isStarting = true;
		this.refusal = '';
		try {
			const started = await startHostUpdate(target);
			this.confirmation = null;
			void this.follow({ fromVersion: started.fromVersion, toVersion: started.toVersion, startedAt: Date.parse(started.startedAt) });
		} catch (failure) {
			this.refusal = refusalOf(failure, text);
			await this.read();
		} finally {
			this.isStarting = false;
		}
	};

	forget = () => {
		this.version = null;
		this.confirmation = null;
		this.refusal = '';
		this.watch = null;
		this.stage = null;
		this.settled = null;
	};

	private follow = async (following: UpdateWatch) => {
		this.watch = following;
		this.settled = null;
		this.stage = { kind: 'installing' };
		while (this.watch === following) {
			const reading = await readHostWhileItMayBeAway();
			if (reading.isAnswered) this.version = reading.version;
			this.stage = stageOfTheUpdate(following, reading, Date.now());
			if (isSettled(this.stage)) {
				this.settled = { watch: following, stage: this.stage };
				this.watch = null;
				return;
			}
			await new Promise((resolve) => setTimeout(resolve, updateCheckMilliseconds));
		}
	};
}

function refusalOf(failure: unknown, text: HostUpdateRefusalText): string {
	if (failure instanceof ToolRefused) return refusalMessageOf(failure.errorCode, text.refusals, failure.message);
	return failure instanceof Error && failure.message ? failure.message : text.startFailed;
}

export const hostUpdate = new HostUpdate();
