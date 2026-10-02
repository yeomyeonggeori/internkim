<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import MemberTable, { type TabledMember } from '$lib/components/member-table.svelte';
	import OrganizationInviteDialog from '../organization/organization-invite-dialog.svelte';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import MessageSquareOffIcon from '@lucide/svelte/icons/message-square-off';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import {
		removeMemberFromCompany,
		resetMemberPassword,
		wipeMemberMessages,
		type MemberInvitation
	} from '$lib/organization/invite-member';
	import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { toast } from 'svelte-sonner';
	import { onMount } from 'svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();
	const tableText = $derived({
		name: text.name,
		email: text.email,
		hireDate: text.hireDate,
		role: text.role,
		empty: text.membersEmpty
	});

	let members = $state<TabledMember[]>([]);
	let isInviteOpen = $state(false);
	let leaving = $state<TabledMember | null>(null);
	let isPurging = $state(false);
	let isRemoving = $state(false);
	let resetting = $state<TabledMember | null>(null);
	let wiping = $state<TabledMember | null>(null);
	let busyMemberID = $state('');
	let issued = $state<MemberInvitation | null>(null);
	let errorMessage = $state('');

	onMount(readTheDirectory);

	async function readTheDirectory() {
		try {
			const directory = await supabaseOrganizationDirectory();
			members = (directory.records ?? []).map((record) => ({
				id: record.memberID ?? record.email,
				name: record.name ?? '',
				email: record.email,
				image: record.image,
				hireDate: record.hireDate,
				role: record.role ?? 'member'
			}));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.membersUnavailable;
		}
	}

	function memberLabel(member: TabledMember): string {
		return member.name || member.email;
	}

	function askAbout(member: TabledMember) {
		leaving = member;
		isPurging = false;
		errorMessage = '';
	}

	async function removeThem() {
		if (!leaving) return;
		isRemoving = true;
		try {
			await removeMemberFromCompany(leaving.id, isPurging);
			leaving = null;
			await readTheDirectory();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.removeFailed;
		} finally {
			isRemoving = false;
		}
	}

	function askAboutPassword(member: TabledMember) {
		resetting = member;
		errorMessage = '';
	}

	async function resetTheirPassword() {
		if (!resetting) return;
		busyMemberID = resetting.id;
		try {
			issued = await resetMemberPassword(resetting.id);
			resetting = null;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.resetPasswordFailed;
		} finally {
			busyMemberID = '';
		}
	}

	function askAboutMessages(member: TabledMember) {
		wiping = member;
		errorMessage = '';
	}

	async function wipeTheirMessages() {
		if (!wiping) return;
		busyMemberID = wiping.id;
		try {
			const wiped = await wipeMemberMessages(wiping.email);
			const count = wiped.buzzDeleted + wiped.mattermostDeleted;
			wiping = null;
			toast.success(text.wipeMessagesDone.replace('{count}', String(count)));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.wipeMessagesFailed;
		} finally {
			busyMemberID = '';
		}
	}
</script>

{#snippet actionsHeader()}
	<Table.Head class="h-10 w-16 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">
		<span class="sr-only">{text.memberActions}</span>
	</Table.Head>
{/snippet}

{#snippet actionsCell(member: TabledMember)}
	<Table.Cell class="text-right">
		{@render memberActions(member)}
	</Table.Cell>
{/snippet}

{#snippet memberActions(member: TabledMember)}
	<div class="flex justify-end">
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						type="button"
						variant="ghost"
						size="icon"
						disabled={busyMemberID === member.id}
						aria-label="{text.memberActions}: {memberLabel(member)}"
					>
						<EllipsisVerticalIcon class="size-4" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end" class="w-48">
				<DropdownMenu.Item onSelect={() => askAboutPassword(member)}>
					<KeyRoundIcon />
					{text.resetPassword}
				</DropdownMenu.Item>
				<DropdownMenu.Item onSelect={() => askAboutMessages(member)}>
					<MessageSquareOffIcon />
					{text.wipeMessages}
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<DropdownMenu.Item variant="destructive" onSelect={() => askAbout(member)}>
					<Trash2Icon />
					{text.removeMember}
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>
{/snippet}

<Card.Root>
	<Card.Content class="grid gap-4">
		<MemberTable {members} text={tableText} extraHeaders={actionsHeader} extraCells={actionsCell} mobileExtra={memberActions} extraColumnCount={1} />
		<div>
			<Button type="button" variant="outline" onclick={() => (isInviteOpen = true)}>
				<UserPlusIcon />
				{text.inviteMember}
			</Button>
		</div>
		{#if errorMessage}
			<p class="text-sm text-destructive">{errorMessage}</p>
		{/if}
	</Card.Content>
</Card.Root>

<OrganizationInviteDialog bind:isOpen={isInviteOpen} onInvited={readTheDirectory} />

<AlertDialog.Root open={leaving !== null} onOpenChange={(open) => !open && (leaving = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.removeMemberTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{text.removeMemberDescription.replace('{name}', leaving ? memberLabel(leaving) : '')}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<div class="flex items-start justify-between gap-4 rounded-lg border p-3">
			<div class="grid gap-1">
				<Label for="purge-{fieldID}">{text.purgeMember}</Label>
				<p class="text-xs text-muted-foreground">{text.purgeMemberDescription}</p>
			</div>
			<Switch id="purge-{fieldID}" bind:checked={isPurging} />
		</div>
		<AlertDialog.Footer>
			<AlertDialog.Cancel type="button" disabled={isRemoving}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action type="button" variant="destructive" disabled={isRemoving} onclick={removeThem}>
				{text.removeMember}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root open={resetting !== null} onOpenChange={(open) => !open && (resetting = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.resetPasswordTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{text.resetPasswordDescription.replace('{name}', resetting ? memberLabel(resetting) : '')}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel type="button" disabled={busyMemberID !== ''}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action type="button" disabled={busyMemberID !== ''} onclick={resetTheirPassword}>
				{text.resetPasswordConfirm}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root open={wiping !== null} onOpenChange={(open) => !open && (wiping = null)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.wipeMessagesTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{text.wipeMessagesDescription.replace('{name}', wiping ? memberLabel(wiping) : '')}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel type="button" disabled={busyMemberID !== ''}>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action
				type="button"
				variant="destructive"
				disabled={busyMemberID !== ''}
				onclick={wipeTheirMessages}
			>
				{text.wipeMessagesConfirm}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<Dialog.Root open={issued !== null} onOpenChange={(open) => !open && (issued = null)}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{text.temporaryPasswordTitle}</Dialog.Title>
			<Dialog.Description>{text.temporaryPasswordDescription}</Dialog.Description>
		</Dialog.Header>
		{#if issued}
			<div class="grid gap-2">
				<p class="text-sm">{issued.email}</p>
				<p class="rounded-lg border bg-muted p-3 font-mono text-lg select-all">{issued.temporaryPassword}</p>
				<p class="text-xs text-muted-foreground">{text.temporaryPasswordHandOver}</p>
			</div>
		{/if}
		<Dialog.Footer>
			<Button onclick={() => (issued = null)}>{text.done}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
