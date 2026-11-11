import { environmentOfPlatform } from '$lib/server/agent-request';
import { adminCallerOf, asMember, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { signedInAccessTokenOf } from '$lib/server/member-request';
import { error } from '@sveltejs/kit';

export type BillingAdmin = { secretKey: string; companySlug: string; customerID: string | null };

export async function billingAdminOf(request: Request, platformEnvironment: unknown, refusal: string): Promise<BillingAdmin> {
	const environment = environmentOfPlatform(platformEnvironment);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');
	const secretKey = environment.STRIPE_SECRET_KEY ?? '';
	if (!secretKey) error(503, 'Stripe billing is not configured');

	const accessToken = await signedInAccessTokenOf(request, plane);
	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, refusal);

	const { data, error: readError } = await controlPlane(plane)
		.from('company')
		.select('slug, stripe_customer_id')
		.eq('id', caller.companyID)
		.single<{ slug: string; stripe_customer_id: string | null }>();
	if (readError) error(500, readError.message);
	return { secretKey, companySlug: data.slug, customerID: data.stripe_customer_id };
}
