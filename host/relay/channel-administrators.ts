import { reasonOf } from './failure';

export const channelCreateCapability = 'person.channel.create';
export const seatAdministratorsPath = '/agent/api/buzz-seat-admins';

export type AdministratorSeating = {
	emailOf: () => Promise<string | null>;
	ask: (requesterEmail: string, channelID: string) => Promise<{ status: number; body: unknown }>;
};

export function createdChannelIDOf(answered: { status: number; body: unknown }): string | null {
	if (answered.status >= 300) return null;
	const body = answered.body;
	if (typeof body !== 'object' || body === null || !('id' in body)) return null;
	return typeof body.id === 'string' && body.id !== '' ? body.id : null;
}

export async function seatAdministratorsInCreatedChannel(seating: AdministratorSeating, channelID: string): Promise<void> {
	try {
		const requesterEmail = await seating.emailOf();
		if (!requesterEmail) throw new Error('the member who opened the channel has no address');
		const answered = await seating.ask(requesterEmail, channelID);
		if (answered.status >= 300) throw new Error(`admind answered ${answered.status}: ${JSON.stringify(answered.body)}`);
	} catch (failure) {
		console.error('administrators were not seated in a new channel', { channelID, reason: reasonOf(failure) });
	}
}
