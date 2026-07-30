<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { toast } from 'svelte-sonner';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
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
		try {
			applyUsersResponse(await fetchUsers(adminBaseURL, text.messages.usersLoadError));
		} catch (error) {
			toast.error(usersErrorMessage(error, text.messages.usersLoadError));
		} finally {
			isLoadingUsers = false;
		}
	}

	async function saveCircle() {
		if (!adminBaseURL || !newCircleID.trim()) return;

		isSavingUser = true;
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
			toast.error(apiErrorMessage(error, text.messages.userSaveError));
		} finally {
			isSavingUser = false;
		}
	}

	async function removeCircle(circleID: string) {
		if (!adminBaseURL || isReservedCircleID(circleID)) return;

		isSavingUser = true;
		try {
			await deleteCircle(adminBaseURL, circleID, text.messages.userRemoveError);
			await loadUsers();
		} catch (error) {
			toast.error(apiErrorMessage(error, text.messages.userRemoveError));
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
			toast.error(usersErrorMessage(error, text.messages.userInviteError));
		} finally {
			isSavingUser = false;
		}
	}

	async function saveUserRecord(record: UserRecord, role: UserRole = record.role): Promise<boolean> {
		if (!fleetID || !adminBaseURL) return false;

		isSavingUser = true;
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
			toast.error(usersErrorMessage(error, text.messages.userSaveError, text.messages.adminAuthRequired));
			return false;
		} finally {
			isSavingUser = false;
		}
	}

	async function submitUserRecord(record: UserRecord, role?: UserRole): Promise<void> {
		await saveUserRecord(record, role);
	}

	async function saveUserNote(record: UserRecord, note: string): Promise<boolean> {
		record.note = note.trim();
		return saveUserRecord(record);
	}

	async function removeEmail(email: string) {
		if (!fleetID || !adminBaseURL) return;

		isSavingUser = true;
		temporaryPasswordResult = null;
		try {
			applyUsersResponse(await removeUser(adminBaseURL, email, text.messages.userRemoveError));
		} catch (error) {
			toast.error(usersErrorMessage(error, text.messages.userRemoveError));
		} finally {
			isSavingUser = false;
		}
	}

	async function resetPassword(record: UserRecord) {
		if (!fleetID) return;
		if (!confirm(text.users.resetPasswordConfirm)) return;

		isSavingUser = true;
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
			toast.error(apiErrorMessage(error, text.users.resetPasswordError));
		} finally {
			isSavingUser = false;
		}
	}
</script>

{#if !isDeviceContext}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.users.title}</Card.Title>
			<Card.Description>{text.users.deviceOnly}</Card.Description>
		</Card.Header>
	</Card.Root>
{:else}
	<Card.Root>
		<Card.Header class="border-b pb-4">
			<Card.Title>{text.users.inviteTitle}</Card.Title>
			<Card.Description>{text.users.inviteDescription}</Card.Description>
			<Card.Action>
				<Badge variant="outline">{userCount()} {text.users.userCount}</Badge>
			</Card.Action>
		</Card.Header>
		<form
			onsubmit={(event) => {
				event.preventDefault();
				addEmail();
			}}
		>
			<Card.Content>
				<div class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
					<Field.Field>
						<Field.Label for="invite-handle">{text.users.handle}</Field.Label>
						<Input id="invite-handle" bind:value={newHandle} placeholder="mohyeong" autocomplete="off" />
					</Field.Field>
					<Field.Field>
						<Field.Label for="invite-name">{text.users.realName}</Field.Label>
						<Input id="invite-name" bind:value={newName} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
					</Field.Field>
					<Field.Field>
						<Field.Label for="invite-email">{text.users.email}</Field.Label>
						<div class="relative">
							<MailIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
							<Input id="invite-email" bind:value={newEmail} type="email" placeholder={text.users.emailPlaceholder} class="pl-9" />
						</div>
					</Field.Field>
					<Field.Field>
						<Field.Label for="invite-hire-date">{text.users.hireDate}</Field.Label>
						<Input id="invite-hire-date" bind:value={newHireDate} type="date" />
					</Field.Field>
					<Field.Field>
						<Field.Label for="invite-role">{text.users.role}</Field.Label>
						<Select.Root type="single" bind:value={newUserRole}>
							<Select.Trigger id="invite-role" class="w-full">
								{userRoleOptions(text, canGrantAdminRole).find((option) => option.value === newUserRole)?.label ?? '-'}
							</Select.Trigger>
							<Select.Content>
								{#each userRoleOptions(text, canGrantAdminRole) as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</Field.Field>
				</div>
			</Card.Content>
			<Card.Footer class="flex-wrap justify-between gap-3">
				<p class="text-muted-foreground text-sm">{text.users.passwordNotice}</p>
				<Button type="submit" disabled={isSavingUser || !newEmail.trim() || !newName.trim() || !isValidHandle(newHandle)}>
					{#if isSavingUser}
						<RefreshCwIcon class="size-4 animate-spin" />
					{:else}
						<PlusIcon />
					{/if}
					{text.users.invite}
				</Button>
			</Card.Footer>
		</form>
	</Card.Root>

	<Card.Root>
		<Card.Header class="border-b pb-4">
			<Card.Title>{text.users.groupTitle}</Card.Title>
			<Card.Description>{text.users.groupDescription}</Card.Description>
		</Card.Header>
		<form
			onsubmit={(event) => {
				event.preventDefault();
				saveCircle();
			}}
		>
			<Card.Content>
				<Field.Group>
					<div class="grid gap-5 md:grid-cols-2">
						<Field.Field>
							<Field.Label for="circle-id">{text.users.groupID}</Field.Label>
							<Input id="circle-id" bind:value={newCircleID} placeholder={text.users.groupIDPlaceholder} autocomplete="off" />
						</Field.Field>
						<Field.Field>
							<Field.Label for="circle-name">{text.users.groupName}</Field.Label>
							<Input id="circle-name" bind:value={newCircleName} placeholder={text.users.groupNamePlaceholder} autocomplete="off" />
						</Field.Field>
					</div>
					<Field.Label>
						<Field.Field orientation="horizontal">
							<Checkbox bind:checked={newCircleMattermostManaged} />
							<Field.Content>
								<Field.Title>{text.users.mattermostManaged}</Field.Title>
							</Field.Content>
						</Field.Field>
					</Field.Label>
					{#if availableCircles.length > 0}
						<div class="flex flex-wrap gap-2">
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
					{/if}
				</Field.Group>
			</Card.Content>
			<Card.Footer class="justify-end">
				<Button type="submit" disabled={isSavingUser || !newCircleID.trim() || isReservedCircleID(newCircleID)}>{text.users.addGroup}</Button>
			</Card.Footer>
		</form>
	</Card.Root>

	<AlertDialog.Root
		open={temporaryPasswordResult !== null}
		onOpenChange={(open) => {
			if (!open) temporaryPasswordResult = null;
		}}
	>
		<AlertDialog.Content class="sm:max-w-md">
			{#if temporaryPasswordResult}
				<AlertDialog.Header>
					<AlertDialog.Title>{text.users.temporaryPasswordTitle}</AlertDialog.Title>
					<AlertDialog.Description>{temporaryPasswordResult.email}</AlertDialog.Description>
				</AlertDialog.Header>
				<div class="flex items-center gap-2">
					<code class="flex-1 rounded-md border bg-muted px-3 py-2 font-mono text-base">{temporaryPasswordResult.password}</code>
					<CopyButton text={temporaryPasswordResult.password} />
				</div>
				<p class="text-muted-foreground text-sm">{text.users.temporaryPasswordNotice}</p>
				<AlertDialog.Footer>
					<AlertDialog.Action onclick={() => (temporaryPasswordResult = null)}>
						{text.users.temporaryPasswordClose}
					</AlertDialog.Action>
				</AlertDialog.Footer>
			{/if}
		</AlertDialog.Content>
	</AlertDialog.Root>

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
				onSaveUser={submitUserRecord}
				onSaveNote={saveUserNote}
			/>
		{/if}
	{/if}
