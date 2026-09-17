import { error } from '@sveltejs/kit';
import { companyComputerName, hostConfigurationSchema, type HostConfiguration, type HostSetupStatus } from '$lib/company/host-setup';
import { issueAgentKey } from './control-plane';
import { callingMember, type CallingMember } from './member-request';
import type { Environment } from './agent-request';

export async function callingHostAdministrator(
	request: Request,
	environment: Environment,
	resolveCaller: typeof callingMember = callingMember
): Promise<CallingMember> {
	const member = await resolveCaller(request, environment);
	if (member.tokenName) error(403, 'sign in to manage the company computer');
	const administrator = await member.caller.from('member')
		.select('is_admin, status').eq('id', member.memberID)
		.single<{ is_admin: boolean; status: string }>();
	if (administrator.error) error(502, administrator.error.message);
	if (!administrator.data?.is_admin || administrator.data.status !== 'active') {
		error(403, 'only an active administrator can connect the company computer');
	}
	return member;
}

export async function companyHostSetupStatus(member: CallingMember): Promise<HostSetupStatus> {
	const company = await member.caller.from('company').select('id, name, slug')
		.eq('id', member.companyID).single<HostConfiguration['company']>();
	if (company.error) error(502, company.error.message);
	const computer = await member.record.from('agent').select('last_seen_at, revoked_at')
		.eq('company_id', member.companyID).eq('name', companyComputerName)
		.maybeSingle<{ last_seen_at: string | null; revoked_at: string | null }>();
	if (computer.error) error(502, computer.error.message);
	return {
		company: company.data,
		hasConfiguration: Boolean(computer.data),
		lastSeenAt: computer.data?.last_seen_at ?? null
	};
}

export async function createHostConfiguration(
	member: CallingMember,
	environment: Environment,
	appURL: string,
	replaceExisting: boolean
): Promise<HostConfiguration> {
	const status = await companyHostSetupStatus(member);
	if (status.hasConfiguration && !replaceExisting) error(409, 'confirm replacing the existing computer connection');
	const configuration = hostConfigurationSchema.omit({ agentKey: true }).safeParse({
		schemaVersion: 1,
		appURL,
		company: status.company,
		centralPlane: { projectURL: environment.SUPABASE_URL, publishableKey: environment.SUPABASE_PUBLISHABLE_KEY },
		gatewayURL: environment.GATEWAY_URL
	});
	if (!configuration.success) error(503, 'the company computer connection is not configured');
	const issued = await issueAgentKey(member.record, member.companyID, companyComputerName, { replaceStanding: replaceExisting });
	return { ...configuration.data, agentKey: issued.apiKey };
}
