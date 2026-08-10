<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { crmAccountTypes, type CRMAccount, type CRMAccountStatus, type CRMAccountType, type CRMImportance } from './crm-types';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		account: CRMAccount | undefined;
		text: CRMText;
		onSave: (account: CRMAccount) => Promise<void>;
		onArchive: (accountID: string) => Promise<void>;
	};

	let { open = $bindable(false), account, text, onSave, onArchive }: Props = $props();
	const accountStatuses: CRMAccountStatus[] = ['prospect', 'active', 'paused'];
	const importanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
	let name = $state('');
	let types = $state<CRMAccountType[]>([]);
	let status = $state<CRMAccountStatus>('prospect');
	let importance = $state<CRMImportance>('medium');
	let ownerName = $state('');
	let ownerEmail = $state('');
	let team = $state('');
	let address = $state('');
	let tags = $state<string[]>([]);
	let description = $state('');
	let lastContactDate = $state('');
	let nextActionDate = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');

	function resetForm(selectedAccount: CRMAccount): void {
		name = selectedAccount.name;
		types = [...selectedAccount.types];
		status = selectedAccount.status;
		importance = selectedAccount.importance;
		ownerName = selectedAccount.ownerName;
		ownerEmail = selectedAccount.ownerEmail;
		team = selectedAccount.team;
		address = selectedAccount.address ?? '';
		tags = [...selectedAccount.tags];
		description = selectedAccount.description;
		lastContactDate = selectedAccount.lastContactDate;
		nextActionDate = selectedAccount.nextActionDate;
		isSaving = false;
		errorMessage = '';
	}

	function setType(accountType: CRMAccountType, checked: boolean): void {
		if (checked) {
			if (!types.includes(accountType)) types = [...types, accountType];
			return;
		}
		types = types.filter((type) => type !== accountType);
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!account || name.trim() === '') return;
		isSaving = true;
		errorMessage = '';
		try {
			await onSave({
				...account,
				name: name.trim(),
				types,
				status,
				importance,
				ownerName: ownerName.trim(),
				address: address.trim() || undefined,
				tags,
				description: description.trim()
			});
			open = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	function archive(): void {
		if (!account) return;
		open = false;
		confirmDelete({
			title: text.archive,
			description: text.archiveConfirm,
			confirm: { text: text.archive },
			cancel: { text: text.cancel },
			onCancel: () => (open = true),
			onConfirm: async () => {
				isSaving = true;
				errorMessage = '';
				try {
					await onArchive(account.id);
					open = false;
				} catch (error) {
					errorMessage = error instanceof Error ? error.message : text.requiredField;
					open = true;
				} finally {
					isSaving = false;
				}
			}
		});
	}

	$effect(() => {
		if (open && account) resetForm(account);
	});
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0 sm:max-w-xl">
		<form onsubmit={save} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12">
				<Sheet.Title>{text.editRelationship}</Sheet.Title>
				<Sheet.Description>{text.editRelationshipDescription}</Sheet.Description>
			</Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5">
				<Field.Group>
					<Field.Field><Field.Label for="crm-edit-account-name">{text.accountName}</Field.Label><Input id="crm-edit-account-name" bind:value={name} required /></Field.Field>
					<Field.Field>
						<Field.Label>{text.type}</Field.Label>
						<div class="grid gap-3 rounded-md border p-3 sm:grid-cols-2">
							{#each crmAccountTypes as accountType (accountType)}
								<label class="flex items-center gap-2 text-sm"><Checkbox checked={types.includes(accountType)} onCheckedChange={(checked) => setType(accountType, checked)} />{text.accountTypes[accountType]}</label>
							{/each}
						</div>
					</Field.Field>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-account-status">{text.status}</Field.Label><Select.Root type="single" value={status} onValueChange={(value) => (status = value as CRMAccountStatus)}><Select.Trigger id="crm-edit-account-status" class="w-full">{text.accountStatuses[status]}</Select.Trigger><Select.Content>{#each accountStatuses as option (option)}<Select.Item value={option} label={text.accountStatuses[option]}>{text.accountStatuses[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<Field.Field><Field.Label for="crm-edit-account-importance">{text.importance}</Field.Label><Select.Root type="single" value={importance} onValueChange={(value) => (importance = value as CRMImportance)}><Select.Trigger id="crm-edit-account-importance" class="w-full">{text.importanceLabels[importance]}</Select.Trigger><Select.Content>{#each importanceOptions as option (option)}<Select.Item value={option} label={text.importanceLabels[option]}>{text.importanceLabels[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-account-owner">{text.owner}</Field.Label><Input id="crm-edit-account-owner" bind:value={ownerName} /></Field.Field>
						<Field.Field><Field.Label for="crm-edit-account-team">{text.team}</Field.Label><Input id="crm-edit-account-team" bind:value={team} disabled /></Field.Field>
					</div>
					<Field.Field><Field.Label for="crm-edit-account-email">{text.ownerEmail}</Field.Label><Input id="crm-edit-account-email" type="email" bind:value={ownerEmail} disabled /></Field.Field>
					<Field.Field><Field.Label for="crm-edit-account-address">{text.address}</Field.Label><Input id="crm-edit-account-address" bind:value={address} placeholder={text.addressPlaceholder} /></Field.Field>
					<Field.Field><Field.Label for="crm-edit-account-tags">{text.tags}</Field.Label><TagsInput id="crm-edit-account-tags" bind:value={tags} placeholder={tags.length === 0 ? text.tagsPlaceholder : undefined} /></Field.Field>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-account-last-contact">{text.lastContact}</Field.Label><Input id="crm-edit-account-last-contact" type="date" bind:value={lastContactDate} disabled /></Field.Field>
						<Field.Field><Field.Label for="crm-edit-account-next-action">{text.nextContactDate}</Field.Label><Input id="crm-edit-account-next-action" type="date" bind:value={nextActionDate} disabled /></Field.Field>
					</div>
					<Field.Field><Field.Label for="crm-edit-account-details">{text.details}</Field.Label><Textarea id="crm-edit-account-details" rows={6} bind:value={description} /></Field.Field>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
				</Field.Group>
			</div>
			<Sheet.Footer class="flex-row justify-between border-t bg-background px-4 py-3">
				<Button type="button" variant="destructive" onclick={archive} disabled={isSaving}>{text.archive}</Button>
				<div class="flex gap-2"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button><Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.save}</Button></div>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
