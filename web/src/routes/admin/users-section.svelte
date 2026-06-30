<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import {
		hasUserCircle,
		normalizeUserCircles,
		toggleUserRecordCircle,
		updateUserRecords,
		type UserRecordChanges
	} from './admin-user-record-changes';
	import {
		apiErrorMessage,
		createCircle,
		createUser,
		deleteCircle,
		fetchUsers,
		isAdminApiStatus,
		removeUser,
		resetUserPassword,
		saveUser
	} from './admin-api';
	import type { AdminPageText, AdminSession, CircleRecord, UserRecord, UserRole, UsersResponse } from './admin-types';
	import TemporaryPasswordPanel from './temporary-password-panel.svelte';
	import UserDirectoryPanel from './user-directory-panel.svelte';
	import UserGroupsPanel from './user-groups-panel.svelte';
	import UserInviteForm from './user-invite-form.svelte';

	type UsersSectionProps = {
		adminBaseURL: string;
		adminSession: AdminSession | null;
		fleetID: string;
		isDeviceContext: boolean;
		text: AdminPageText;
		onUserChanged: () => Promise<void> | void;
	};

	type TemporaryPasswordResult = {
		email: string;
		password: string;
	};

	let { adminBaseURL, adminSession, fleetID, isDeviceContext, text, onUserChanged }: UsersSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let userRecords = $state<UserRecord[]>([]);
	let availableCircles = $state<CircleRecord[]>([{ circleID: 'staff', displayName: 'Staff' }]);
	let newHandle = $state('');
	let newName = $state('');
	let newEmail = $state('');
	let newHireDate = $state('');
	let newUserRole = $state<UserRole>('member');
	let newCircleID = $state('');
	let newCircleName = $state('');
	let newCircleMattermostManaged = $state(true);
	let temporaryPasswordResult = $state<TemporaryPasswordResult | null>(null);
	let isLoadingUsers = $state(false);
	let isSavingUser = $state(false);
	let errorMessage = $state('');

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadUsers();
	});

	$effect(() => {
		if (temporaryPasswordResult) return;
		if (!adminSession?.temporaryPassword || !adminSession.temporaryPasswordEmail) return;
		temporaryPasswordResult = {
			email: adminSession.temporaryPasswordEmail,
			password: adminSession.temporaryPassword
		};
	});

	function userCount() {
		return userRecords.length;
	}

	function adminCount() {
		return userRecords.filter((record) => record.role === 'admin').length;
	}

	function userRoleOptions(): { value: UserRole; label: string }[] {
		return [
			{ value: 'member', label: text.users.member },
			{ value: 'admin', label: text.users.admin }
		];
	}

	function visibleCircles() {
		return availableCircles.filter((circle) => circle.circleID !== 'admin');
	}

	function normalizeHandle(handle: string) {
		return handle.trim().toLowerCase();
	}

	function isValidHandle(handle: string) {
		return /^[a-z][a-z0-9._-]{2,21}$/.test(normalizeHandle(handle));
	}

	function isValidUserRecord(record: UserRecord) {
		return isValidHandle(record.handle) && !!record.name?.trim() && !!record.email.trim();
	}

	function userRecordsByHireDate() {
		return [...userRecords].sort((first, second) => {
			if (first.hireDate || second.hireDate) {
				if (!first.hireDate) return 1;
				if (!second.hireDate) return -1;
				if (first.hireDate !== second.hireDate) return first.hireDate.localeCompare(second.hireDate);
			}
			return (first.name || first.email).localeCompare(second.name || second.email);
		});
	}

	function applyUsersResponse(response: UsersResponse) {
		availableCircles = response.availableCircles?.length ? response.availableCircles : [{ circleID: 'staff', displayName: 'Staff' }];
		if (response.records) {
			userRecords = response.records.map((record) => ({
				...record,
				name: record.name ?? '',
				hireDate: record.hireDate ?? '',
				note: record.note ?? '',
				circles: normalizeUserCircles(record.circles, record.role)
			}));
		} else {
			userRecords = (response.users ?? []).map((email, index) => ({
				userID: `legacy-${index}`,
				handle: email.split('@')[0]?.toLowerCase() ?? '',
				email,
				hireDate: '',
				note: '',
				role: index === 0 ? 'admin' : 'member',
				circles: index === 0 ? ['staff', 'admin'] : ['staff']
			}));
		}
		if (response.temporaryPassword && response.temporaryPasswordEmail) {
			temporaryPasswordResult = {
				email: response.temporaryPasswordEmail,
				password: response.temporaryPassword
			};
		}
	}

	function toggleUserCircle(record: UserRecord, circleID: string) {
		userRecords = toggleUserRecordCircle(userRecords, record.email, circleID);
	}

	function updateUserRecord(record: UserRecord, changes: UserRecordChanges) {
		userRecords = updateUserRecords(userRecords, record.email, changes);
	}

	async function saveUserNote(record: UserRecord, note: string): Promise<boolean> {
		const trimmedNote = note.trim();
		const nextRecord = { ...record, note: trimmedNote };
		const didSave = await saveUserRecord(nextRecord);
		if (didSave) {
			userRecords = updateUserRecords(userRecords, record.email, { note: trimmedNote });
		}
		return didSave;
	}

	function usersErrorMessage(error: unknown, fallbackMessage: string, forbiddenMessage: string = text.messages.adminAuthRequiredOnDevice) {
		if (isAdminApiStatus(error, 403)) return forbiddenMessage;
		return apiErrorMessage(error, fallbackMessage);
	}

	async function loadUsers() {
		if (!fleetID || !adminBaseURL) return;

		isLoadingUsers = true;
		errorMessage = '';
		try {
			applyUsersResponse(await fetchUsers(adminBaseURL, text.messages.usersLoadError));
		} catch (error) {
			errorMessage = usersErrorMessage(error, text.messages.usersLoadError);
		} finally {
			isLoadingUsers = false;
		}
	}

	async function saveCircle() {
		if (!adminBaseURL || !newCircleID.trim()) return;

		isSavingUser = true;
		errorMessage = '';
		try {
			await createCircle(
				adminBaseURL,
				{
					circleID: newCircleID.trim().toLowerCase(),
					displayName: newCircleName.trim() || newCircleID.trim(),
					isMattermostManaged: newCircleMattermostManaged
				},
				text.messages.userSaveError
			);
			newCircleID = '';
			newCircleName = '';
			newCircleMattermostManaged = true;
			await loadUsers();
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userSaveError);
		} finally {
			isSavingUser = false;
		}
	}

	async function removeCircle(circleID: string) {
		if (!adminBaseURL || circleID === 'staff') return;

		isSavingUser = true;
		errorMessage = '';
		try {
			await deleteCircle(adminBaseURL, circleID, text.messages.userRemoveError);
			await loadUsers();
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userRemoveError);
		} finally {
			isSavingUser = false;
		}
	}

	async function addEmail() {
		const email = newEmail.trim().toLowerCase();
		const handle = normalizeHandle(newHandle);
		const name = newName.trim();
		if (!email || !handle || !name || !fleetID || !adminBaseURL) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			applyUsersResponse(await createUser(adminBaseURL, { handle, name, email, hireDate: newHireDate, role: newUserRole }, text.messages.userInviteError));
			await onUserChanged();
			newHandle = '';
			newName = '';
			newEmail = '';
			newHireDate = '';
			newUserRole = 'member';
		} catch (error) {
			errorMessage = usersErrorMessage(error, text.messages.userInviteError);
		} finally {
			isSavingUser = false;
		}
	}

	async function saveUserRecord(record: UserRecord, role: UserRole = record.role): Promise<boolean> {
		if (!fleetID || !adminBaseURL) return false;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			applyUsersResponse(await saveUser(
				adminBaseURL,
				{
					userID: record.userID,
					handle: normalizeHandle(record.handle),
					name: record.name?.trim() ?? '',
					hireDate: record.hireDate ?? '',
					note: record.note?.trim() ?? '',
					email: record.email,
					role,
					circles: normalizeUserCircles(record.circles, role),
					mattermostUserID: record.mattermostUserID,
					mattermostUsername: record.mattermostUsername,
					status: record.status
				},
				text.messages.userSaveError
			));
			return true;
		} catch (error) {
			errorMessage = usersErrorMessage(error, text.messages.userSaveError, text.messages.adminAuthRequired);
			return false;
		} finally {
			isSavingUser = false;
		}
	}

	async function removeEmail(email: string) {
		if (!fleetID || !adminBaseURL) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			applyUsersResponse(await removeUser(adminBaseURL, email, text.messages.userRemoveError));
		} catch (error) {
			errorMessage = usersErrorMessage(error, text.messages.userRemoveError);
		} finally {
			isSavingUser = false;
		}
	}

	async function resetPassword(record: UserRecord) {
		if (!fleetID) return;
		if (!confirm(text.users.resetPasswordConfirm)) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			const response = await resetUserPassword(adminBaseURL, record.email, text.users.resetPasswordError);
			if (response.temporaryPassword && response.temporaryPasswordEmail) {
				temporaryPasswordResult = {
					email: response.temporaryPasswordEmail,
					password: response.temporaryPassword
				};
			}
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.users.resetPasswordError);
		} finally {
			isSavingUser = false;
		}
	}
</script>

<div class="flex flex-wrap items-center justify-between gap-3">
	<div>
		<h2 class="text-base font-semibold">{text.users.title}</h2>
		<p class="text-muted-foreground text-sm">
			{text.users.description}
		</p>
	</div>
	<Badge variant="outline">{userCount()} {text.users.userCount}</Badge>
</div>

{#if !isDeviceContext}
	<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
		{text.users.deviceOnly}
	</p>
{:else}
	<UserInviteForm
		text={text.users}
		bind:handle={newHandle}
		bind:name={newName}
		bind:email={newEmail}
		bind:hireDate={newHireDate}
		bind:role={newUserRole}
		roleOptions={userRoleOptions()}
		isSaving={isSavingUser}
		isHandleValid={isValidHandle(newHandle)}
		onSubmit={addEmail}
	/>

	<p class="text-muted-foreground text-sm">
		{text.users.passwordNotice}
	</p>

	<UserGroupsPanel
		text={text.users}
		circles={availableCircles}
		bind:circleID={newCircleID}
		bind:circleName={newCircleName}
		bind:isMattermostManaged={newCircleMattermostManaged}
		isSaving={isSavingUser}
		onSave={saveCircle}
		onRemove={removeCircle}
	/>

	{#if temporaryPasswordResult}
		<TemporaryPasswordPanel text={text.users} email={temporaryPasswordResult.email} password={temporaryPasswordResult.password} />
	{/if}

	{#if errorMessage}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
	{/if}

	<UserDirectoryPanel
		records={userRecordsByHireDate()}
		visibleCircles={visibleCircles()}
		text={text.users}
		adminCount={adminCount()}
		isLoading={isLoadingUsers}
		isSaving={isSavingUser}
		{hasUserCircle}
		{isValidUserRecord}
		onRecordChange={updateUserRecord}
		onToggleCircle={toggleUserCircle}
		onSaveNote={saveUserNote}
		onSave={saveUserRecord}
		onResetPassword={resetPassword}
		onRemove={removeEmail}
	/>
{/if}
