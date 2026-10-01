//   bun run web/scripts/claim-fleet.ts --company <uuid> --fleet <fleet id>

import { claimFleetForCompany, companyOfFleet, controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const fleetID = argument('fleet')?.trim().toLowerCase();
if (!companyID || !fleetID) throw new Error('pass --company <uuid> --fleet <fleet id>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? '',
});

await claimFleetForCompany(client, companyID, fleetID);
console.log(`${fleetID} now belongs to ${await companyOfFleet(client, fleetID)}`);
