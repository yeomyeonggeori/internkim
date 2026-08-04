// Brings a company's work locations across with the colours it already uses.
//   bun run web/scripts/import-work-locations.ts --file <attendance.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceLocation = { name: string; color?: string; isDefault?: boolean };

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!file || !companyID) throw new Error('pass --file <attendance.json> --company <uuid>');

const document = JSON.parse(await Bun.file(file).text()) as { locations?: DeviceLocation[] };
const deviceLocations = document.locations ?? [];
if (deviceLocations.length === 0) throw new Error('the export carries no work locations');

// The first one is what a clock-in with no location becomes, so the default
// leads and the rest keep the order they were registered in.
const ordered = [...deviceLocations].sort((left, right) => Number(right.isDefault) - Number(left.isDefault));
const locations = ordered.map((location) => ({
	name: location.name,
	...(location.color ? { color: location.color } : {}),
}));

console.log(locations.map((location) => `${location.name}${location.color ? ` ${location.color}` : ''}`).join(', '));
if (!shouldApply) {
	console.log('pass --apply to write');
	process.exit(0);
}

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { error } = await client.from('company').update({ work_locations: locations }).eq('id', companyID);
if (error) throw new Error(error.message);
console.log(`wrote ${locations.length} work locations`);
