type FetchImplementation = (input: Parameters<typeof fetch>[0], init?: Parameters<typeof fetch>[1]) => ReturnType<typeof fetch>;

export function createMockFetch(implementation: FetchImplementation): typeof fetch {
	return Object.assign(implementation, { preconnect() {} });
}
