import { callCompanyApp } from './host-bridge';

// A company browser is served by Pages, where admind's own /agent/api route
// answers with the app's 404 page.
export async function centralBuzzRelayURL(): Promise<string> {
	try {
		const answer = await callCompanyApp({ capability: 'person.buzz.relay' });
		if (answer.status !== 200) {
			console.warn('buzz relay address answered', answer.status);
			return '';
		}
		return relayURLOf(answer.body);
	} catch (error) {
		console.warn('buzz relay address did not answer', error);
		return '';
	}
}

function relayURLOf(body: unknown): string {
	if (typeof body !== 'object' || body === null || !('relayURL' in body)) return '';
	const relayURL = body.relayURL;
	return typeof relayURL === 'string' ? relayURL.trim() : '';
}
