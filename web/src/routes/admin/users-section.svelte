<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import XIcon from '@lucide/svelte/icons/x';
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
	import { adminSessionRole } from './admin-role-policy';
	import UsersDirectory from './users-directory.svelte';
	import {
		isReservedCircleID,
		isValidHandle,
		normalizeHandle,
		normalizeUserCircles,
		userRoleOptions
	} from './users-section-policy';
	import type { AdminPageText, AdminSession, CircleRecord, UserRecord, UserRole, UsersResponse } from './admin-types';

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
	const currentAdminRole = $derived(adminSessionRole(adminSession));
	const canGrantAdminRole = $derived(currentAdminRole === 'admin');

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

	function applyUsersResponse(response: UsersResponse) {
		availableCircles = response.availableCircles?.length ? response.availableCircles : [{ circleID: 'staff', displayName: 'Staff' }];
		if (response.records) {
			userRecords = response.records.map((record) => ({
				...record,
				name: record.name ?? '',
				hireDate: record.hireDate ?? '',
				circles: normalizeUserCircles(record.circles, record.role)
			}));
		} else {
			userRecords = (response.users ?? []).map((email, index) => ({
				userID: `legacy-${index}`,
				handle: email.split('@')[0]?.toLowerCase() ?? '',
				email,
				hireDate: '',
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
		if (!adminBaseURL || isReservedCircleID(circleID)) return;

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

	async function saveUserNote(record: UserRecord, note: string): Promise<boolean> {
		record.note = note.trim();
		return saveUserRecord(record);
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
	<Card.Root>
		<Card.Header class="gap-1">
			<Card.Title class="text-sm">{text.users.inviteTitle}</Card.Title>
			<Card.Description>{text.users.inviteDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			<form
				class="grid gap-3 lg:grid-cols-[minmax(120px,0.8fr)_minmax(150px,1fr)_minmax(210px,1.3fr)_150px_120px_auto] lg:items-end"
				onsubmit={(event) => {
					event.preventDefault();
					addEmail();
				}}
			>
				<label class="grid gap-1.5">
					<Label>{text.users.handle}</Label>
					<Input bind:value={newHandle} placeholder="mohyeong" autocomplete="off" />
				</label>
				<label class="grid gap-1.5">
					<Label>{text.users.realName}</Label>
					<Input bind:value={newName} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
				</label>
				<label class="grid gap-1.5">
					<Label>{text.users.email}</Label>
					<div class="relative">
						<MailIcon class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
						<Input bind:value={newEmail} type="email" placeholder={text.users.emailPlaceholder} class="pl-9" />
					</div>
				</label>
				<label class="grid gap-1.5">
					<Label>{text.users.hireDate}</Label>
					<Input bind:value={newHireDate} type="date" />
				</label>
				<label class="grid gap-1.5">
						<Label>{text.users.role}</Label>
						<Select.Root type="single" bind:value={newUserRole}>
							<Select.Trigger class="w-full">
								{userRoleOptions(text, canGrantAdminRole).find((option) => option.value === newUserRole)?.label ?? '-'}
							</Select.Trigger>
							<Select.Content>
								{#each userRoleOptions(text, canGrantAdminRole) as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
					</Select.Root>
				</label>
				<Button type="submit" disabled={isSavingUser || !newEmail.trim() || !newName.trim() || !isValidHandle(newHandle)} class="gap-2">
					{#if isSavingUser}
						<RefreshCwIcon class="size-4 animate-spin" />
					{:else}
						<PlusIcon class="size-4" />
					{/if}
					{text.users.invite}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>

	<p class="text-muted-foreground text-sm">
		{text.users.passwordNotice}
	</p>

	<Card.Root>
		<Card.Header class="gap-1">
			<Card.Title class="text-sm">{text.users.groupTitle}</Card.Title>
			<Card.Description>{text.users.groupDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			<form
				class="grid gap-3 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end"
				onsubmit={(event) => {
					event.preventDefault();
					saveCircle();
				}}
			>
				<label class="grid gap-1.5">
					<Label>{text.users.groupID}</Label>
					<Input bind:value={newCircleID} placeholder={text.users.groupIDPlaceholder} autocomplete="off" />
				</label>
				<label class="grid gap-1.5">
					<Label>{text.users.groupName}</Label>
					<Input bind:value={newCircleName} placeholder={text.users.groupNamePlaceholder} autocomplete="off" />
				</label>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={newCircleMattermostManaged} />
					{text.users.mattermostManaged}
				</label>
				<Button type="submit" disabled={isSavingUser || !newCircleID.trim() || isReservedCircleID(newCircleID)}>{text.users.addGroup}</Button>
			</form>
			<div class="mt-3 flex flex-wrap gap-2">
				{#each availableCircles as circle (circle.circleID)}
					<Badge variant="outline" class="gap-2">
						{circle.displayName || circle.circleID}
						{#if !isReservedCircleID(circle.circleID)}
							<button
								type="button"
								class="text-muted-foreground hover:text-destructive"
								onclick={() => removeCircle(circle.circleID)}
								aria-label={`${text.users.remove} ${circle.displayName || circle.circleID}`}
							>
								<XIcon class="size-3" />
							</button>
						{/if}
					</Badge>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>

	{#if temporaryPasswordResult}
		<div class="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950">
			<div class="flex flex-wrap items-start justify-between gap-3">
				<div>
					<p class="font-semibold">{text.users.temporaryPasswordTitle}: {temporaryPasswordResult.email} / {temporaryPasswordResult.password}</p>
					<p class="mt-1">{text.users.temporaryPasswordNotice}</p>
					<code class="mt-3 block rounded-md bg-white px-3 py-2 font-mono text-base">{temporaryPasswordResult.password}</code>
				</div>
				<CopyButton text={temporaryPasswordResult.password} />
			</div>
		</div>
	{/if}

	{#if errorMessage}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
	{/if}

		{#if isLoadingUsers}
			<p class="text-muted-foreground text-sm">{text.users.loading}</p>
		{:else if userRecords.length === 0}
			<p class="text-muted-foreground text-sm">{text.users.empty}</p>
		{:else}
			<UsersDirectory
				{availableCircles}
				{canGrantAdminRole}
				{isSavingUser}
				{text}
				bind:userRecords
				onRemoveUser={removeEmail}
				onResetPassword={resetPassword}
				onSaveUser={saveUserRecord}
				onSaveNote={saveUserNote}
			/>
		{/if}
	{/if}
