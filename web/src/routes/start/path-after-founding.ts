import { wantsCompanyPrefix } from '$lib/company-path';

const setupPath = '/settings/setup';

export function pathAfterFounding(returnPath: string): string {
	if (!returnPath) return setupPath;
	const [pathname = ''] = returnPath.split(/[?#]/);
	return wantsCompanyPrefix(pathname) ? setupPath : returnPath;
}
