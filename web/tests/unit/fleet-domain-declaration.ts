import { readFileSync } from 'node:fs';

export function declaredZone(): string {
	const source = readFileSync(new URL('../../../internal/fleetdomain/fleetdomain.go', import.meta.url), 'utf8');
	const declaration = source.match(/var defaultZone = "([^"]+)"/);
	if (!declaration) throw new Error('internal/fleetdomain/fleetdomain.go no longer declares defaultZone');
	return declaration[1];
}
