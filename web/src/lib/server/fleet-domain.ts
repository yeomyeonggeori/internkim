export const defaultZone = 'intern.kim';

export function docsHost(zone: string = defaultZone): string {
	return `docs.${zone}`;
}
