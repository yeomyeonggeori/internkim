import { createClient } from '@supabase/supabase-js';
import { planeCredentialsOf } from '$lib/server/control-plane';
import type { ToolPlace } from './company';
import { RecordRefusedTheWrite } from './tasks';

export async function sendGuestSignInEmail(place: ToolPlace, email: string, landingPath: string) {
	const plane = planeCredentialsOf(place.environment);
	if (!plane) throw new RecordRefusedTheWrite('the central plane is not configured', 503);
	const sender = createClient(plane.projectURL, plane.publishableKey, {
		auth: { persistSession: false, autoRefreshToken: false }
	});
	const { error } = await sender.auth.signInWithOtp({ email, options: {
		shouldCreateUser: true, emailRedirectTo: `${place.appAddress}${landingPath}`
	} });
	if (error) throw new RecordRefusedTheWrite(error.message, error.status ?? 502);
}
