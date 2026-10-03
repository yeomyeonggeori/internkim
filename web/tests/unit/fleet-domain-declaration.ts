import { readFileSync } from 'node:fs';

export function declaredZone(): string {
	const source = readFileSync(new URL('../../../internal/companyzone/companyzone.go', import.meta.url), 'utf8');
	const declaration = source.match(/var defaultZone = "([^"]+)"/);
	if (!declaration) throw new Error('internal/companyzone/companyzone.go no longer declares defaultZone');
	return declaration[1];
}
