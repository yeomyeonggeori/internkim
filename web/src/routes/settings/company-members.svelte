<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import MemberTable, { type TabledMember } from '$lib/components/member-table.svelte';
	import OrganizationInviteDialog from '../organization/organization-invite-dialog.svelte';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { removeMemberFromCompany } from '$lib/organization/invite-member';
	import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
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
</script>

{#snippet removeHeader()}
	<Table.Head class="h-10 w-16 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">
		<span class="sr-only">{text.removeMember}</span>
	</Table.Head>
{/snippet}

{#snippet removeCell(member: TabledMember)}
	<Table.Cell class="text-right">
		<Button
			type="button"
			variant="ghost"
			size="icon"
			aria-label="{text.removeMember}: {member.name || member.email}"
			onclick={() => askAbout(member)}
		>
			<Trash2Icon class="size-4" />
		</Button>
	</Table.Cell>
{/snippet}

<Card.Root>
	<Card.Content class="grid gap-4">
		<MemberTable {members} text={tableText} extraHeaders={removeHeader} extraCells={removeCell} extraColumnCount={1} />
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
				{text.removeMemberDescription.replace('{name}', leaving?.name || leaving?.email || '')}
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
