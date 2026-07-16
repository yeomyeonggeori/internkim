import { describe, expect, mock, test } from 'bun:test';
import type { UserRecord } from '../../../src/lib/orgchart/types';
import { adminText } from '../../../src/routes/admin/text';
import { orgchartDirectoryText } from '../../../src/routes/orgchart/text';

let localeValue: 'ko' | 'en' = 'ko';

mock.module('../../../src/lib/i18n/locale.svelte', () => ({
	currentLocale: {
		get value() {
			return localeValue;
		}
	},
	setLocale(nextLocale: 'ko' | 'en') {
		localeValue = nextLocale;
	}
}));

const { currentLocale, setLocale } = await import('../../../src/lib/i18n/locale.svelte');
const { OrgchartDirectoryController } = await import('../../../src/routes/orgchart/orgchart-directory-controller.svelte');

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

function createController(): InstanceType<typeof OrgchartDirectoryController> {
	Object.assign(globalThis, {
		$state<Value>(value: Value): Value {
			return value;
		},
		$derived<Value>(value: Value): Value {
			return value;
		}
	});
	try {
		return new OrgchartDirectoryController('/admin/api', orgchartDirectoryText.ko, adminText.ko);
	} finally {
		Reflect.deleteProperty(globalThis, '$state');
		Reflect.deleteProperty(globalThis, '$derived');
	}
}

describe('orgchart directory controller', () => {
	test('recalculates organization order when the current locale changes', () => {
		const originalLocale = currentLocale.value;
		const controller = createController();
		controller.groups = [
			{ id: 'korean', name: '가' },
			{ id: 'english', name: 'A' }
		];
		controller.records = [
			userRecord({ userID: 'korean', primaryGroupID: 'korean', groupIDs: ['korean'] }),
			userRecord({ userID: 'english', primaryGroupID: 'english', groupIDs: ['english'] })
		];
		controller.visibleRecords = controller.records;

		try {
			setLocale('ko');
			expect(controller.organizationSections.map((section) => section.id)).toEqual(['korean', 'english']);

			setLocale('en');
			expect(controller.organizationSections.map((section) => section.id)).toEqual(['english', 'korean']);
		} finally {
			setLocale(originalLocale);
		}
	});
});
