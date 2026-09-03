import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { notificationCategories as browserCategories } from '../../../src/lib/notifications/categories';
import { choicesFromStored } from '../../../src/lib/server/public-api/record/notifications';
import {
	notificationCategories as sharedCategories,
	readNotificationSettings as sharedRead
} from '../../../../supabase/functions/_shared/categories.ts';

type CatalogTool = {
	name: string;
	inputSchema: { properties?: Record<string, { items?: { enum?: string[] } }> };
};

const generatedCatalogPath = new URL(
	'../../../../pkg/capabilityprotocol/generated/capability-tools.json',
	import.meta.url
);

function categoriesTheCatalogPublishes(): readonly string[] {
	const catalog: { tools: CatalogTool[] } = JSON.parse(readFileSync(generatedCatalogPath, 'utf8'));
	const tool = catalog.tools.find((named) => named.name === 'notification_settings_set');
	const published = tool?.inputSchema.properties?.turnOn?.items?.enum;
	if (!published) throw new Error('the generated catalog publishes no notification categories');
	return published;
}

describe('every copy of the categories says what the catalog does', () => {
	test('the browser names the same categories in the same order', () => {
		const browser: readonly string[] = browserCategories;
		expect(browser).toEqual(categoriesTheCatalogPublishes());
	});

	test('the delivery side names the same categories in the same order', () => {
		const delivery: readonly string[] = sharedCategories;
		expect(delivery).toEqual(categoriesTheCatalogPublishes());
	});

	test('the record and the delivery side read a stored choice the same way', () => {
		expect(sharedRead(undefined).categories).toEqual(choicesFromStored(undefined));
		expect(sharedRead({ mail: true, task: false }).categories).toEqual(
			choicesFromStored({ mail: true, task: false })
		);
	});
});
