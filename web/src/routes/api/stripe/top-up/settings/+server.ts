import { environmentOfPlatform } from '$lib/server/agent-request';
import { adminCallerOf, asMember, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { signedInAccessTokenOf } from '$lib/server/member-request';
import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import type { RequestHandler } from './$types';

const topUpSetting = z.object({
	enabled: z.boolean(),
	thresholdCredits: z.number().int().positive(),
	amount: z.number().int().positive()
}).strict();

type SettingRow = { enabled: boolean; threshold_credits: number; amount: number };
type TopUpRow = { status: string; failure: string | null; created_at: string; settled_at: string | null };

async function adminOf(request: Request, platform: App.Platform | undefined) {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');
	const accessToken = await signedInAccessTokenOf(request, plane);
	const caller = asMember(plane, accessToken);
	const admin = await adminCallerOf(caller);
	if (!admin) error(403, 'only an admin sets the credit top-up');
	return { plane, caller, companyID: admin.companyID };
}

export const GET: RequestHandler = async ({ request, platform }) => {
	const { caller, companyID } = await adminOf(request, platform);
	const [setting, latest] = await Promise.all([
		caller.from('company_credit_top_up_setting').select('enabled, threshold_credits, amount').eq('company_id', companyID).maybeSingle<SettingRow>(),
		caller.from('company_credit_top_up').select('status, failure, created_at, settled_at').eq('company_id', companyID)
			.order('created_at', { ascending: false }).limit(1).maybeSingle<TopUpRow>()
	]);
	if (setting.error) error(500, `reading the top-up setting of company ${companyID}: ${setting.error.message}`);
	if (latest.error) error(500, `reading the latest top-up of company ${companyID}: ${latest.error.message}`);
	return json({
		setting: setting.data
			? { enabled: setting.data.enabled, thresholdCredits: setting.data.threshold_credits, amount: setting.data.amount }
			: null,
		latestTopUp: latest.data
			? { status: latest.data.status, failure: latest.data.failure, createdAt: latest.data.created_at, settledAt: latest.data.settled_at }
			: null
	});
};

export const PUT: RequestHandler = async ({ request, platform }) => {
	const { plane, companyID } = await adminOf(request, platform);
	const asked = topUpSetting.safeParse(await request.json().catch(() => null));
	if (!asked.success) error(400, `the top-up setting is not valid: ${z.prettifyError(asked.error)}`);

	const { error: writeError } = await controlPlane(plane).rpc('company_credit_top_up_configured', {
		target_company: companyID,
		is_enabled: asked.data.enabled,
		threshold: asked.data.thresholdCredits,
		charged_amount: asked.data.amount
	});
	if (writeError) error(500, `writing the top-up setting of company ${companyID}: ${writeError.message}`);
	return json({ setting: asked.data });
};
