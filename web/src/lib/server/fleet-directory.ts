import { error } from '@sveltejs/kit';
import { companyOfFleet, controlPlane } from './control-plane';
import type { CompanyDirectory } from './member-directory';
import type { Environment } from './agent-request';

export async function fleetDirectory(environment: Environment, fleetID: string): Promise<CompanyDirectory> {
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
	if (!projectURL || !serviceRoleKey) throw error(500, 'the central plane is not configured');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const companyID = await companyOfFleet(client, fleetID);
	if (!companyID) throw error(404, 'this fleet belongs to no company yet');
	return { client, companyID };
}
