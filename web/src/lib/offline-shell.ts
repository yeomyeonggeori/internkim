export type Fetched = {
	method: string;
	url: string;
};

export function isShippedFile(fetched: Fetched, appOrigin: string, shipped: Set<string>): boolean {
	if (fetched.method !== 'GET') return false;
	const url = readURL(fetched.url);
	if (!url) return false;
	if (url.origin !== appOrigin) return false;
	if (url.search) return false;
	return shipped.has(url.pathname);
}

function readURL(address: string): URL | null {
	try {
		return new URL(address);
	} catch {
		return null;
	}
}
