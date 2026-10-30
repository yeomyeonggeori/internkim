import { invokeTool } from '$lib/public-api-call';

const recordPageSize = 100;

export async function everyPageOf<Page extends { nextCursor?: string }>(
	toolName: string,
	input: Record<string, unknown>
): Promise<Page[]> {
	const pages: Page[] = [];
	let cursor: string | undefined;
	do {
		const page = await invokeTool<Page>(toolName, {
			...input,
			limit: recordPageSize,
			...(cursor ? { cursor } : {})
		});
		pages.push(page);
		cursor = page.nextCursor;
	} while (cursor);
	return pages;
}
