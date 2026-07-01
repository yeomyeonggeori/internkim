<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Table from '$lib/components/ui/table';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import XIcon from '@lucide/svelte/icons/x';
	import type { AdminPageText, CircleRecord, UserRecord, UserRole } from './admin-types';
	import UserNoteControl from './user-note-control.svelte';
	import {
		canManageUserRecord,
		hasUserCircle,
		isValidUserRecord,
		nextUserCircles,
		sortUserRecordsByHireDate,
		userAdminCount,
		userRoleLabel,
		visibleUserCircles
	} from './users-section-policy';

	type UsersDirectoryProps = {
		availableCircles: CircleRecord[];
		canGrantAdminRole: boolean;
		isSavingUser: boolean;
		text: AdminPageText;
		userRecords: UserRecord[];
		onRemoveUser: (email: string) => Promise<void> | void;
		onResetPassword: (record: UserRecord) => Promise<void> | void;
		onSaveUser: (record: UserRecord, role?: UserRole) => Promise<void> | void;
		onSaveNote: (record: UserRecord, note: string) => Promise<boolean>;
	};

	let {
		availableCircles,
		canGrantAdminRole,
		isSavingUser,
		text,
		userRecords = $bindable(),
		onRemoveUser,
		onResetPassword,
		onSaveUser,
		onSaveNote
	}: UsersDirectoryProps = $props();

	function userCanManage(record: UserRecord): boolean {
		return canManageUserRecord(canGrantAdminRole, record);
	}

	function toggleUserCircle(record: UserRecord, circleID: string): void {
		if (circleID === 'staff') return;
		record.circles = nextUserCircles(record, circleID);
	}
</script>

<Card.Root class="overflow-hidden">
	<Card.Header class="flex-row items-center justify-between gap-3 border-b">
		<div>
			<Card.Title class="text-sm">{text.users.directoryTitle}</Card.Title>
			<Card.Description>{text.users.directoryDescription}</Card.Description>
		</div>
		<Badge variant="secondary">{userAdminCount(userRecords)} {text.users.adminCount}</Badge>
	</Card.Header>
	<Card.Content class="p-0">
		<div class="relative hidden overflow-x-auto lg:block">
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
					{#each sortUserRecordsByHireDate(userRecords) as record (record.email)}
						<Table.Row>
							<Table.Cell>
								<div class="flex min-w-0 items-center gap-3">
									<PersonAvatar name={record.name} email={record.email} class="size-9" />
									<div class="min-w-0">
										<div class="flex min-w-0 items-center gap-1.5">
											<p class="truncate text-sm font-medium">{record.email}</p>
											<UserNoteControl note={record.note} text={text.users} isSaving={isSavingUser} onSave={(note) => onSaveNote(record, note)} />
										</div>
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
								<Input
									bind:value={record.handle}
									placeholder={text.users.handlePlaceholder}
									autocomplete="off"
									disabled={!userCanManage(record)}
								/>
							</Table.Cell>
							<Table.Cell>
								<Input
									bind:value={record.name}
									placeholder={text.users.realNamePlaceholder}
									autocomplete="off"
									disabled={!userCanManage(record)}
								/>
							</Table.Cell>
							<Table.Cell>
								<Input bind:value={record.hireDate} type="date" disabled={!userCanManage(record)} />
							</Table.Cell>
							<Table.Cell>
								<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{userRoleLabel(text, record.role)}</Badge>
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-wrap gap-1.5">
									{#each visibleUserCircles(availableCircles) as circle (circle.circleID)}
										<Button
											type="button"
											variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
											size="sm"
											disabled={circle.circleID === 'staff' || isSavingUser || !userCanManage(record)}
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
			<div
				aria-hidden="true"
				class="pointer-events-none absolute inset-y-0 right-0 w-5 bg-gradient-to-l from-background/95 to-transparent"
			></div>
		</div>
		<div class="grid gap-0 lg:hidden">
			{#each sortUserRecordsByHireDate(userRecords) as record (record.email)}
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
						<div class="flex shrink-0 items-center gap-2">
							<UserNoteControl note={record.note} text={text.users} isSaving={isSavingUser} onSave={(note) => onSaveNote(record, note)} />
							<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{userRoleLabel(text, record.role)}</Badge>
						</div>
					</div>
					<div class="grid gap-3 sm:grid-cols-3">
						<label class="grid gap-1.5">
							<Label>{text.users.handle}</Label>
							<Input
								bind:value={record.handle}
								placeholder={text.users.handlePlaceholder}
								autocomplete="off"
								disabled={!userCanManage(record)}
							/>
						</label>
						<label class="grid gap-1.5">
							<Label>{text.users.realName}</Label>
							<Input
								bind:value={record.name}
								placeholder={text.users.realNamePlaceholder}
								autocomplete="off"
								disabled={!userCanManage(record)}
							/>
						</label>
						<label class="grid gap-1.5">
							<Label>{text.users.hireDate}</Label>
							<Input bind:value={record.hireDate} type="date" disabled={!userCanManage(record)} />
						</label>
					</div>
					<div class="flex flex-wrap gap-1.5">
						{#each visibleUserCircles(availableCircles) as circle (circle.circleID)}
							<Button
								type="button"
								variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
								size="sm"
								disabled={circle.circleID === 'staff' || isSavingUser || !userCanManage(record)}
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

{#snippet UserActions(record: UserRecord, layout: 'desktop' | 'mobile')}
	<div class={layout === 'desktop' ? 'flex justify-end gap-2' : 'grid gap-2 sm:grid-cols-4'}>
		<Button
			variant="outline"
			size="sm"
			disabled={isSavingUser || !userCanManage(record) || !isValidUserRecord(record)}
			onclick={() => onSaveUser(record)}
		>
			{text.users.save}
		</Button>
		<Button
			class="gap-2"
			variant="outline"
			size="sm"
			disabled={isSavingUser || !userCanManage(record) || !isValidUserRecord(record)}
			onclick={() => onResetPassword(record)}
		>
			<RefreshCwIcon class="size-4" />
			{text.users.resetPassword}
		</Button>
		{#if record.role === 'admin' && canGrantAdminRole}
			<Button
				variant="outline"
				size="sm"
				disabled={isSavingUser || userAdminCount(userRecords) <= 1 || !isValidUserRecord(record)}
				onclick={() => onSaveUser(record, 'member')}
			>
				{text.users.makeMember}
			</Button>
		{:else if canGrantAdminRole}
			<Button variant="outline" size="sm" disabled={isSavingUser || !isValidUserRecord(record)} onclick={() => onSaveUser(record, 'admin')}>
				{text.users.makeAdmin}
			</Button>
		{/if}
		<Button
			variant="ghost"
			size={layout === 'desktop' ? 'icon-sm' : 'sm'}
			disabled={isSavingUser || !userCanManage(record) || (record.role === 'admin' && userAdminCount(userRecords) <= 1)}
			onclick={() => onRemoveUser(record.email)}
			aria-label={text.users.remove}
		>
			<XIcon class="size-4" />
			{#if layout === 'mobile'}
				<span>{text.users.remove}</span>
			{/if}
		</Button>
	</div>
{/snippet}
