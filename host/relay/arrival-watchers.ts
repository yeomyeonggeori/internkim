import { reasonOf } from './failure';
import type { SupabaseClient } from '@supabase/supabase-js';
import type { ActorCredential } from './forward';

export const arrivalsWatchCapability = 'person.arrivals.watch';
export const arrivalsPath = '/arrived';
export const arrivalsRenewalMilliseconds = 2 * 60_000;

export type ArrivalWatchersDependencies = {
	activeMemberIDs: () => Promise<string[]>;
	credentialOf: (memberID: string) => Promise<ActorCredential | null>;
	askChatd: (capability: string, body: Record<string, unknown>) => Promise<{ status: number; body: unknown }>;
	arrivalsURL: string;
	typingURL: string;
	report: (line: string) => void;
	now: () => number;
};

export type ArrivalWatchRenewal = { watched: number; withoutCredential: number; refusals: string[] };

export type ScheduleRenewal = (milliseconds: number, renew: () => Promise<void>) => void;

const firstRetryMilliseconds = 1_000;

export class ArrivalWatchers {
	private readonly watchedAt = new Map<string, number>();
	private readonly watching = new Map<string, Promise<void>>();

	constructor(private readonly dependencies: ArrivalWatchersDependencies) {}

	async renewEveryone(): Promise<ArrivalWatchRenewal> {
		const renewal: ArrivalWatchRenewal = { watched: 0, withoutCredential: 0, refusals: [] };
		for (const memberID of await this.dependencies.activeMemberIDs()) {
			const actor = await this.dependencies.credentialOf(memberID);
			if (!actor) {
				renewal.withoutCredential += 1;
				continue;
			}
			const refusal = await this.watch(memberID, actor);
			if (refusal) renewal.refusals.push(refusal);
			else renewal.watched += 1;
		}
		return renewal;
	}

	watchOnceTheyAct(memberID: string, actor: ActorCredential): Promise<void> {
		const inFlight = this.watching.get(memberID);
		if (inFlight) return inFlight;
		if (this.isWatched(memberID)) return Promise.resolve();
		const watching = this.watch(memberID, actor)
			.then((refusal) => {
				if (refusal) this.dependencies.report(`arrival watch for ${refusal}`);
			})
			.catch((error) => {
				this.dependencies.report(`arrival watch for member ${memberID} not started: ${reasonOf(error)}`);
			})
			.finally(() => this.watching.delete(memberID));
		this.watching.set(memberID, watching);
		return watching;
	}

	report(line: string): void {
		this.dependencies.report(line);
	}

	private isWatched(memberID: string): boolean {
		const watchedAt = this.watchedAt.get(memberID);
		return watchedAt !== undefined && this.dependencies.now() - watchedAt < arrivalsRenewalMilliseconds;
	}

	private async watch(memberID: string, actor: ActorCredential): Promise<string | null> {
		const answer = await this.dependencies.askChatd(arrivalsWatchCapability, {
			actor,
			arrivalsURL: this.dependencies.arrivalsURL,
			typingURL: this.dependencies.typingURL
		});
		if (answer.status >= 300) return `member ${memberID}: chatd answered ${answer.status}: ${JSON.stringify(answer.body)}`;
		this.watchedAt.set(memberID, this.dependencies.now());
		return null;
	}
}

export async function activeMemberIDsOf(client: SupabaseClient, companyID: string): Promise<string[]> {
	const members = await client
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq('status', 'active')
		.returns<{ id: string }[]>();
	if (members.error) throw new Error(`the active members of company ${companyID} could not be read: ${members.error.message}`);
	return members.data.map((member) => member.id);
}

export function keepWatchingArrivals(watchers: ArrivalWatchers, schedule: ScheduleRenewal): Promise<void> {
	let retryMilliseconds = firstRetryMilliseconds;
	let lastOutcome = '';
	const report = (outcome: string) => {
		if (outcome !== lastOutcome) watchers.report(outcome);
		lastOutcome = outcome;
	};
	const renew = async (): Promise<void> => {
		const isComplete = await renewedCompletely(watchers, report);
		const delay = isComplete ? arrivalsRenewalMilliseconds : retryMilliseconds;
		retryMilliseconds = isComplete ? firstRetryMilliseconds : Math.min(retryMilliseconds * 2, arrivalsRenewalMilliseconds);
		schedule(delay, renew);
	};
	return renew();
}

async function renewedCompletely(watchers: ArrivalWatchers, report: (outcome: string) => void): Promise<boolean> {
	try {
		const renewal = await watchers.renewEveryone();
		report(
			`arrival watches: ${renewal.watched} watched, ${renewal.withoutCredential} without a credential, refused: ${renewal.refusals.join('; ') || 'none'}`
		);
		return renewal.refusals.length === 0;
	} catch (error) {
		report(`arrival watches not renewed: ${reasonOf(error)}`);
		return false;
	}
}
