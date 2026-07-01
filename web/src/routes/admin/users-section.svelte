<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
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

	function userRoleOptions() {
		return [
			{ value: 'member', label: text.users.member },
			{ value: 'operationsAdmin', label: text.users.operationsAdmin },
			{ value: 'admin', label: text.users.admin }
		];
	}

	function userRoleLabel(role: UserRole) {
		return userRoleOptions().find((option) => option.value === role)?.label ?? role;
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

	function normalizeUserCircles(circles: string[] | undefined, role: UserRole) {
		const result = new Set(['staff', ...(circles ?? []).map((circle) => circle.trim().toLowerCase()).filter(Boolean)]);
		if (role === 'admin') result.add('admin');
		return [...result];
	}

	function hasUserCircle(record: UserRecord, circleID: string) {
		return normalizeUserCircles(record.circles, record.role).includes(circleID);
	}

	function toggleUserCircle(record: UserRecord, circleID: string) {
		if (circleID === 'staff') return;
		const current = new Set(normalizeUserCircles(record.circles, record.role));
		if (current.has(circleID)) current.delete(circleID);
		else current.add(circleID);
		record.circles = normalizeUserCircles([...current], record.role);
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

	async function saveUserRecord(record: UserRecord, role: UserRole = record.role) {
		if (!fleetID || !adminBaseURL) return;

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
					email: record.email,
					role,
					circles: normalizeUserCircles(record.circles, role),
					mattermostUserID: record.mattermostUserID,
					mattermostUsername: record.mattermostUsername,
					status: record.status
				},
				text.messages.userSaveError
			));
		} catch (error) {
			errorMessage = usersErrorMessage(error, text.messages.userSaveError, text.messages.adminAuthRequired);
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
							{userRoleOptions().find((option) => option.value === newUserRole)?.label ?? '-'}
						</Select.Trigger>
						<Select.Content>
							{#each userRoleOptions() as option (option.value)}
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
				<Button type="submit" disabled={isSavingUser || !newCircleID.trim()}>{text.users.addGroup}</Button>
			</form>
			<div class="mt-3 flex flex-wrap gap-2">
				{#each availableCircles as circle (circle.circleID)}
					<Badge variant="outline" class="gap-2">
						{circle.displayName || circle.circleID}
						{#if circle.circleID !== 'staff'}
							<button type="button" class="text-muted-foreground hover:text-destructive" onclick={() => removeCircle(circle.circleID)}>
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
		<Card.Root class="overflow-hidden">
			<Card.Header class="flex-row items-center justify-between gap-3 border-b">
				<div>
					<Card.Title class="text-sm">{text.users.directoryTitle}</Card.Title>
					<Card.Description>{text.users.directoryDescription}</Card.Description>
				</div>
				<Badge variant="secondary">{adminCount()} {text.users.adminCount}</Badge>
			</Card.Header>
			<Card.Content class="p-0">
				<div class="hidden overflow-x-auto lg:block">
					<Table.Root class="min-w-[1260px]">
						<Table.Header class="bg-muted/40">
							<Table.Row class="hover:bg-transparent">
								<Table.Head class="w-[250px]">{text.users.person}</Table.Head>
								<Table.Head class="w-[160px]">{text.users.handle}</Table.Head>
								<Table.Head class="w-[180px]">{text.users.realName}</Table.Head>
								<Table.Head class="w-[150px]">{text.users.hireDate}</Table.Head>
								<Table.Head class="w-[110px]">{text.users.role}</Table.Head>
								<Table.Head class="w-[220px]">{text.users.groups}</Table.Head>
								<Table.Head class="text-right">{text.users.actions}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each userRecordsByHireDate() as record (record.email)}
								<Table.Row>
									<Table.Cell>
										<div class="flex min-w-0 items-center gap-3">
											<PersonAvatar name={record.name} email={record.email} class="size-9" />
											<div class="min-w-0">
												<p class="truncate text-sm font-medium">{record.email}</p>
												<p class="truncate text-xs text-muted-foreground">
													{record.mattermostUsername ? `Mattermost: ${record.mattermostUsername}` : text.users.noMattermost}
												</p>
												{#if record.isIncomplete}
													<p class="text-xs text-destructive">{text.users.incomplete}</p>
												{/if}
											</div>
										</div>
									</Table.Cell>
									<Table.Cell>
										<Input bind:value={record.handle} placeholder={text.users.handlePlaceholder} autocomplete="off" />
									</Table.Cell>
									<Table.Cell>
										<Input bind:value={record.name} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
									</Table.Cell>
									<Table.Cell>
										<Input bind:value={record.hireDate} type="date" />
									</Table.Cell>
									<Table.Cell>
										<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{userRoleLabel(record.role)}</Badge>
									</Table.Cell>
									<Table.Cell>
										<div class="flex flex-wrap gap-1.5">
											{#each visibleCircles() as circle (circle.circleID)}
												<Button
													type="button"
													variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
													size="sm"
													disabled={circle.circleID === 'staff' || isSavingUser}
													onclick={() => toggleUserCircle(record, circle.circleID)}
													title={circle.isMattermostManaged ? text.users.mattermostManaged : ''}
												>
													{circle.displayName || circle.circleID}
												</Button>
											{/each}
										</div>
									</Table.Cell>
									<Table.Cell>
										{@render UserActions(record, 'desktop')}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
				<div class="grid gap-0 lg:hidden">
					{#each userRecordsByHireDate() as record (record.email)}
						<div class="grid gap-3 border-b p-4 last:border-b-0">
							<div class="flex min-w-0 items-start justify-between gap-3">
								<div class="flex min-w-0 items-center gap-3">
									<PersonAvatar name={record.name} email={record.email} class="size-10" />
									<div class="min-w-0">
										<p class="truncate text-sm font-medium">{record.name || record.email}</p>
										<p class="truncate text-xs text-muted-foreground">{record.email}</p>
										{#if record.mattermostUsername}
											<p class="truncate text-xs text-muted-foreground">Mattermost: {record.mattermostUsername}</p>
										{/if}
									</div>
								</div>
								<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{userRoleLabel(record.role)}</Badge>
							</div>
							<div class="grid gap-3 sm:grid-cols-3">
								<label class="grid gap-1.5">
									<Label>{text.users.handle}</Label>
									<Input bind:value={record.handle} placeholder={text.users.handlePlaceholder} autocomplete="off" />
								</label>
								<label class="grid gap-1.5">
									<Label>{text.users.realName}</Label>
									<Input bind:value={record.name} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
								</label>
								<label class="grid gap-1.5">
									<Label>{text.users.hireDate}</Label>
									<Input bind:value={record.hireDate} type="date" />
								</label>
							</div>
							<div class="flex flex-wrap gap-1.5">
								{#each visibleCircles() as circle (circle.circleID)}
									<Button
										type="button"
										variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
										size="sm"
										disabled={circle.circleID === 'staff' || isSavingUser}
										onclick={() => toggleUserCircle(record, circle.circleID)}
									>
										{circle.displayName || circle.circleID}
									</Button>
								{/each}
							</div>
							{#if record.isIncomplete}
								<p class="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">{text.users.incomplete}</p>
							{/if}
							{@render UserActions(record, 'mobile')}
						</div>
					{/each}
				</div>
			</Card.Content>
		</Card.Root>
	{/if}
{/if}

{#snippet UserActions(record: UserRecord, layout: 'desktop' | 'mobile')}
	<div class={layout === 'desktop' ? 'flex justify-end gap-2' : 'grid gap-2 sm:grid-cols-4'}>
		<Button variant="outline" size="sm" disabled={isSavingUser || !isValidUserRecord(record)} onclick={() => saveUserRecord(record)}>
			{text.users.save}
		</Button>
		<Button
			class="gap-2"
			variant="outline"
			size="sm"
			disabled={isSavingUser || !isValidUserRecord(record)}
			onclick={() => resetPassword(record)}
		>
			<RefreshCwIcon class="size-4" />
			{text.users.resetPassword}
		</Button>
		{#if record.role === 'admin'}
			<Button
				variant="outline"
				size="sm"
				disabled={isSavingUser || adminCount() <= 1 || !isValidUserRecord(record)}
				onclick={() => saveUserRecord(record, 'member')}
			>
				{text.users.makeMember}
			</Button>
		{:else}
			<Button variant="outline" size="sm" disabled={isSavingUser || !isValidUserRecord(record)} onclick={() => saveUserRecord(record, 'admin')}>
				{text.users.makeAdmin}
			</Button>
		{/if}
		<Button
			variant="ghost"
			size={layout === 'desktop' ? 'icon-sm' : 'sm'}
			disabled={isSavingUser || (record.role === 'admin' && adminCount() <= 1)}
			onclick={() => removeEmail(record.email)}
			aria-label={text.users.remove}
		>
			<XIcon class="size-4" />
			{#if layout === 'mobile'}
				<span>{text.users.remove}</span>
			{/if}
		</Button>
	</div>
{/snippet}
