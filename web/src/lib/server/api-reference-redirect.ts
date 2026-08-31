// The API reference has one home. These two addresses served copies of the same
// generated document and drifted apart from it, so they now point at the home.
const referenceHome = 'https://docs.intern.kim/docs/api';

export function apiReferenceHomeFor(pathname: string): string | null {
	if (pathname === '/api-docs' || pathname.startsWith('/api-docs/')) return referenceHome;
	const document = pathname.match(/^\/openapi\/(en|ko)\.json$/);
	if (document) return `https://docs.intern.kim/openapi/${document[1]}.json`;
	return null;
}
