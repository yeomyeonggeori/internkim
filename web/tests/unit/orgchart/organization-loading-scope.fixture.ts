import { beforeEach, expect, mock, test } from 'bun:test';
import { ToolRefused } from '../../../src/lib/tool-answer';
import { adminText } from '../../../src/routes/admin/text';
import { organizationDirectoryText } from '../../../src/routes/organization/text';
import { forgetLastSeenDirectory, lastSeenDirectory, rememberDirectory } from '../../../src/routes/organization/organization-last-seen';
import type { UsersResponse } from '../../../src/lib/organization/types';

Object.assign(globalThis, { $state: <Value>(value: Value) => value, $derived: <Value>(value: Value) => value });
const directory: UsersResponse = { records: [{ memberID: 'sample-member', name: '이샘플', email: 'sample@example.com', handle: 'sample' }], availableGroups: [] };
let read: () => Promise<UsersResponse>;
let role: () => Promise<string>;
mock.module('../../../src/routes/organization/organization-api', () => ({ fetchOrganizationDirectory: () => read(), organizationApiErrorMessage: (error: unknown, fallback: string) => error instanceof Error ? error.message : fallback, saveOrganizationGroups: async () => directory, saveOrganizationProfiles: async () => directory, saveOwnOrganizationProfile: async () => ({}) }));
mock.module('$lib/supabase-session', () => ({ isSupabaseConfigured: () => true, supabaseMemberRole: () => role() }));
mock.module('$lib/signed-in-email', () => ({ signedInEmail: async () => 'sample@example.com' }));
const { OrganizationDirectoryController } = await import('../../../src/routes/organization/organization-directory-controller.svelte');
const controllerFor = (scope: string) => new OrganizationDirectoryController('/admin/api', organizationDirectoryText.ko, adminText.ko, () => 'ko', scope);

beforeEach(() => { forgetLastSeenDirectory(); read = async () => directory; role = async () => 'admin'; });

test('only the same resolved company, member and role can reuse a directory snapshot', () => {
	rememberDirectory('company-a/member-a/admin', { records: directory.records ?? [], groups: [] });
	expect(controllerFor('company-a/member-a/admin').records).toEqual(directory.records ?? []);
	for (const scope of ['company-b/member-a/admin', 'company-a/member-b/admin', 'company-a/member-a/member', '']) {
		const controller = controllerFor(scope);
		expect(controller.records).toEqual([]);
		expect(controller.isLoading).toBe(true);
	}
});

test('a disposed directory read cannot repopulate the last-seen cache', async () => {
	const gate = Promise.withResolvers<UsersResponse>();
	read = () => gate.promise;
	const controller = controllerFor('company-a/member-a/admin');
	const loading = controller.loadDirectory();
	controller.dispose();
	gate.resolve(directory);
	await loading;
	expect(controller.records).toEqual([]);
	expect(lastSeenDirectory('company-a/member-a/admin')).toBeNull();
});

test('an old denied read does not clear a newer completed same-scope request', async () => {
	const gate = Promise.withResolvers<UsersResponse>();
	read = () => gate.promise;
	const controller = controllerFor('company-a/member-a/admin');
	const older = controller.loadDirectory();
	read = async () => directory;
	await controller.loadDirectory();
	const latestRecords = structuredClone(controller.records);
	gate.reject(new ToolRefused('Old read denied', 'denied', 403));
	await older;
	expect(controller.records).toEqual(latestRecords);
	expect(controller.errorMessage).toBe('');
	expect(lastSeenDirectory('company-a/member-a/admin')?.records).toEqual(latestRecords);
});

test('a current denied read discards selection, edits and the matching cached snapshot', async () => {
	const controller = controllerFor('company-a/member-a/admin');
	await controller.loadDirectory();
	controller.selectedUserID = 'sample-member';
	controller.editingUserID = 'sample-member';
	controller.isConfirmingDiscard = true;
	controller.isAddingGroup = true;
	controller.organizationEdit.begin([]);
	read = async () => { throw new ToolRefused('Access refused', 'denied', 403); };
	await controller.loadDirectory();
	expect(controller.records).toEqual([]);
	expect(controller.selectedUserID).toBe('');
	expect(controller.editingUserID).toBe('');
	expect(controller.isConfirmingDiscard).toBe(false);
	expect(controller.isAddingGroup).toBe(false);
	expect(controller.organizationEdit.isEditing).toBe(false);
	expect(lastSeenDirectory('company-a/member-a/admin')).toBeNull();
});

test('a role answer arriving after a directory denial cannot re-enable management', async () => {
	const gate = Promise.withResolvers<string>();
	role = () => gate.promise;
	read = async () => { throw new ToolRefused('Access refused', 'denied', 401); };
	const controller = controllerFor('company-a/member-a/admin');
	const loading = controller.load();
	await Promise.resolve();
	gate.resolve('admin');
	await loading;
	expect(controller.canManage).toBe(false);
});
