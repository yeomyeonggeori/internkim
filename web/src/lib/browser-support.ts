export interface BrowserScope {
	AbortSignal: object;
	Array: { prototype: object };
	Map: object;
	URL: object;
	structuredClone?: unknown;
}

const requiredFeatures: readonly ((scope: BrowserScope) => boolean)[] = [
	(scope) => 'toSorted' in scope.Array.prototype,
	(scope) => 'toReversed' in scope.Array.prototype,
	(scope) => 'findLast' in scope.Array.prototype,
	(scope) => 'groupBy' in scope.Map,
	(scope) => 'canParse' in scope.URL,
	(scope) => 'timeout' in scope.AbortSignal,
	(scope) => typeof scope.structuredClone === 'function'
];

export function isBrowserOutdated(scope: BrowserScope = globalThis): boolean {
	return requiredFeatures.some((isPresent) => !isPresent(scope));
}
