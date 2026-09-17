const loopbackHosts = new Set(['localhost', '127.0.0.1', '[::1]']);

export function hostOf(address: string): string {
	return URL.canParse(address) ? new URL(address).host : address;
}

export function returnsToThisComputer(address: string): boolean {
	if (!URL.canParse(address)) return false;
	const destination = new URL(address);
	if (destination.protocol === 'https:') return false;
	if (destination.protocol === 'http:') return loopbackHosts.has(destination.hostname);
	return true;
}
