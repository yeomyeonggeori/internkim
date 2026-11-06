import type { Locator } from '@playwright/test';

type DocumentHolder = { locator(selector: string): Locator };

export async function waitForHydration(document: DocumentHolder): Promise<void> {
	await document.locator('html[data-hydrated]').waitFor({ state: 'attached' });
}
