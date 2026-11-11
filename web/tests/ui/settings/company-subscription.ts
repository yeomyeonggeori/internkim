import { centralPlaneAdminClient, exampleCompanyID } from '../support/central-test-utils';

export const subscriptionID = `sub_ui_${exampleCompanyID}`;
export const customerID = `cus_ui_${exampleCompanyID}`;

export async function subscribeTheCompany(): Promise<void> {
	const client = centralPlaneAdminClient();
	const paid = await client.rpc('company_subscription_invoice_paid', {
		target_company: exampleCompanyID,
		target_event: `evt_ui_${exampleCompanyID}_${Date.now()}`,
		target_customer: customerID,
		target_subscription: subscriptionID,
		granted_credits: 40000,
		opens_period: true,
		period_ends_at: new Date(Date.now() + 20 * 86_400_000).toISOString()
	});
	if (paid.error) throw new Error(paid.error.message);
	const changed = await client.rpc('company_subscription_changed', {
		target_company: exampleCompanyID,
		target_customer: customerID,
		target_subscription: subscriptionID,
		target_status: 'active',
		status_at: new Date().toISOString()
	});
	if (changed.error) throw new Error(changed.error.message);
}

export async function unsubscribeTheCompany(): Promise<void> {
	const client = centralPlaneAdminClient();
	const held = await client.from('company').select('rules').eq('id', exampleCompanyID).single();
	if (held.error) throw new Error(held.error.message);
	const rules = { ...(held.data.rules ?? {}) };
	delete rules.creditPolicy;
	const company = await client.from('company').update({
		stripe_customer_id: null,
		stripe_subscription_id: null,
		subscription_status: null,
		subscription_ends_at: null,
		subscription_credits: 0,
		permanent_credits: 0,
		rules
	}).eq('id', exampleCompanyID);
	if (company.error) throw new Error(company.error.message);
	const members = await client.from('member').update({ credits_used: 0 }).eq('company_id', exampleCompanyID);
	if (members.error) throw new Error(members.error.message);
}
