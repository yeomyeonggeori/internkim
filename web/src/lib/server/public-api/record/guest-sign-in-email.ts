import { createClient } from '@supabase/supabase-js';
import { controlPlane, planeCredentialsOf, type PlaneCredentials } from '$lib/server/control-plane';
import type { ToolPlace } from './company';
import { RecordRefusedTheWrite } from './tasks';

export type GuestInvitation = {
	email: string;
	landingPath: string;
	companyName: string;
	inviterName: string;
	locale: string;
};

function requiredPlane(place: ToolPlace): PlaneCredentials {
	const plane = planeCredentialsOf(place.environment);
	if (!plane) throw new RecordRefusedTheWrite('the central plane is not configured', 503);
	return plane;
}

export async function sendGuestSignInEmail(place: ToolPlace, email: string, landingPath: string) {
	const plane = requiredPlane(place);
	const sender = createClient(plane.projectURL, plane.publishableKey, {
		auth: { persistSession: false, autoRefreshToken: false }
	});
	const { error } = await sender.auth.signInWithOtp({ email, options: {
		shouldCreateUser: true, emailRedirectTo: `${place.appAddress}${landingPath}`
	} });
	if (error) throw new RecordRefusedTheWrite(error.message, error.status ?? 502);
}

export async function sendGuestInvitationEmail(place: ToolPlace, invitation: GuestInvitation) {
	const { error } = await controlPlane(requiredPlane(place)).auth.admin.inviteUserByEmail(invitation.email, {
		redirectTo: `${place.appAddress}${invitation.landingPath}`,
		data: { companyName: invitation.companyName, inviterName: invitation.inviterName, locale: invitation.locale }
	});
	if (!error) return;
	if (error.code !== 'email_exists') throw new RecordRefusedTheWrite(error.message, error.status ?? 502);
	await sendGuestSignInEmail(place, invitation.email, invitation.landingPath);
}
