import { supabase } from './supabase';
import { centralBuzzIdentityTransport } from './buzz-identity-central';
import {
	BuzzIdentityNotEnrolledError,
	enrollBuzzIdentity,
	unlockBuzzIdentity,
	type BuzzUnlockFactor
} from './buzz-identity-session';

// Signing in to Supabase proves who somebody is and says nothing about their
// Buzz key: the key is sealed, and only the factor they just used opens it.
// A company member has never had one, so the first sign-in mints it — before
// this, the connect dialog on a company host had nothing to show.
export async function unlockCentralBuzzIdentity(factor: BuzzUnlockFactor): Promise<string> {
	const client = supabase();
	const { data } = await client.auth.getUser();
	const accountID = data.user?.id;
	if (!accountID) throw new Error('sign in first');

	const transport = centralBuzzIdentityTransport(client, accountID);
	try {
		return await unlockBuzzIdentity(transport, factor);
	} catch (error) {
		if (!(error instanceof BuzzIdentityNotEnrolledError)) throw error;
		return enrollBuzzIdentity(transport, factor);
	}
}
