import { reasonOf } from './failure';

export const channelOwnerAddCapability = 'person.channel.owner.add';

export type SeatedOwner = { conversationID: string; externalID: string };

export type SeatedOwnerMuting = {
	memberIDOf: (externalID: string) => Promise<string | null>;
	muteFor: (memberID: string, conversationID: string) => Promise<void>;
};

export function seatedOwnerOf(
	body: Record<string, unknown>,
	answered: { status: number; body: unknown }
): SeatedOwner | null {
	if (answered.status >= 300) return null;
	if (!isNewToChannel(answered.body)) return null;
	const conversationID = typeof body.conversationID === 'string' ? body.conversationID : '';
	const externalID = typeof body.externalID === 'string' ? body.externalID : '';
	if (!conversationID || !externalID) return null;
	return { conversationID, externalID };
}

export function seatedOwnerMuter(muting: SeatedOwnerMuting): (seated: SeatedOwner) => Promise<void> {
	return (seated) =>
		muteSeatedOwner(muting, seated).catch((failure) => {
			console.error('a seated owner was not muted', { ...seated, reason: reasonOf(failure) });
		});
}

async function muteSeatedOwner(muting: SeatedOwnerMuting, seated: SeatedOwner): Promise<void> {
	const memberID = await muting.memberIDOf(seated.externalID);
	if (!memberID) throw new Error('nobody in this company holds that messenger account');
	await muting.muteFor(memberID, seated.conversationID);
}

function isNewToChannel(body: unknown): boolean {
	return typeof body === 'object' && body !== null && 'newToChannel' in body && body.newToChannel === true;
}
