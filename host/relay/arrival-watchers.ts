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
	report: (line: string) => void;
};

export type ArrivalWatchRenewal = { watched: number; withoutCredential: number; refusals: string[] };

export async function renewArrivalWatches(dependencies: ArrivalWatchersDependencies): Promise<ArrivalWatchRenewal> {
	const renewal: ArrivalWatchRenewal = { watched: 0, withoutCredential: 0, refusals: [] };
	for (const memberID of await dependencies.activeMemberIDs()) {
		const actor = await dependencies.credentialOf(memberID);
		if (!actor) {
			renewal.withoutCredential += 1;
			continue;
		}
		const answer = await dependencies.askChatd(arrivalsWatchCapability, {
			actor,
			arrivalsURL: dependencies.arrivalsURL
		});
		if (answer.status < 300) {
			renewal.watched += 1;
			continue;
		}
		renewal.refusals.push(`member ${memberID}: chatd answered ${answer.status}: ${JSON.stringify(answer.body)}`);
	}
	return renewal;
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

export function keepWatchingArrivals(dependencies: ArrivalWatchersDependencies): void {
	let isRenewing = false;
	const renew = async () => {
		if (isRenewing) return;
		isRenewing = true;
		try {
			const renewal = await renewArrivalWatches(dependencies);
			if (renewal.refusals.length > 0 || renewal.withoutCredential > 0) {
				dependencies.report(
					`arrival watches: ${renewal.watched} watched, ${renewal.withoutCredential} without a credential, refused: ${renewal.refusals.join('; ') || 'none'}`
				);
			}
		} catch (error) {
			dependencies.report(`arrival watches not renewed: ${error instanceof Error ? error.message : String(error)}`);
		} finally {
			isRenewing = false;
		}
	};
	void renew();
	setInterval(() => void renew(), arrivalsRenewalMilliseconds);
}
