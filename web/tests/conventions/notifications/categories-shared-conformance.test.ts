import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	attendanceReaches as browserReaches,
	notificationCategories as browserCategories
} from '../../../src/lib/notifications/categories';
import { attendanceReachFromStored, choicesFromStored } from '../../../src/lib/server/public-api/record/notifications';
import {
	attendanceReaches as sharedReaches,
	notificationCategories as sharedCategories,
	readNotificationSettings as sharedRead
} from '../../../../supabase/functions/_shared/categories.ts';

type CatalogTool = {
	name: string;
	inputSchema: { properties?: Record<string, { enum?: string[]; items?: { enum?: string[] } }> };
};

const generatedCatalogPath = new URL(
	'../../../../pkg/capabilityprotocol/generated/capability-tools.json',
	import.meta.url
);

function settingsSetProperties(): NonNullable<CatalogTool['inputSchema']['properties']> {
	const catalog: { tools: CatalogTool[] } = JSON.parse(readFileSync(generatedCatalogPath, 'utf8'));
	const tool = catalog.tools.find((named) => named.name === 'notification_settings_set');
	const properties = tool?.inputSchema.properties;
	if (!properties) throw new Error('the generated catalog publishes no notification_settings_set input');
	return properties;
}

function categoriesTheCatalogPublishes(): readonly string[] {
	const published = settingsSetProperties().turnOn?.items?.enum;
	if (!published) throw new Error('the generated catalog publishes no notification categories');
	return published;
}

function reachesTheCatalogPublishes(): readonly string[] {
	const published = settingsSetProperties().attendanceReach?.enum;
	if (!published) throw new Error('the generated catalog publishes no attendance reaches');
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

	test('the browser and the delivery side name the same attendance reaches', () => {
		const browser: readonly string[] = browserReaches;
		const delivery: readonly string[] = sharedReaches;
		expect(browser).toEqual(reachesTheCatalogPublishes());
		expect(delivery).toEqual(reachesTheCatalogPublishes());
	});

	test('the record and the delivery side read a stored reach the same way', () => {
		for (const stored of [undefined, {}, { attendanceReach: 'team' }, { attendanceReach: 'nobody' }]) {
			expect(sharedRead(stored).attendanceReach).toBe(attendanceReachFromStored(stored));
		}
	});

	test('the record and the delivery side read a stored choice the same way', () => {
		expect(sharedRead(undefined).categories).toEqual(choicesFromStored(undefined));
		expect(sharedRead({ mail: true, task: false }).categories).toEqual(
			choicesFromStored({ mail: true, task: false })
		);
	});
});
