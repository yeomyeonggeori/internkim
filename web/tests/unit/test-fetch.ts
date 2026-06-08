export function createFetchMock(handler: (...parameters: Parameters<typeof fetch>) => ReturnType<typeof fetch>): typeof fetch {
	const originalPreconnect = globalThis.fetch.preconnect;
	const preconnect: typeof fetch.preconnect = (...parameters) => originalPreconnect?.(...parameters);
	return Object.assign(handler, { preconnect });
}
