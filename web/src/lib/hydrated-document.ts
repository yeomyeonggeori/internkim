export function markDocumentHydrated(): void {
	document.documentElement.dataset.hydrated = '';
}

export function isHydratingDocument(): boolean {
	return typeof document !== 'undefined' && !('hydrated' in document.documentElement.dataset);
}
