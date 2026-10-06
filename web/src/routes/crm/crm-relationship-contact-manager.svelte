<script lang="ts">
	import * as Empty from '$lib/components/ui/empty';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import type { CRMContact } from './crm-types';
	import type { CRMText } from './text';

	type Props = {
		organizationID: string;
		contacts: CRMContact[];
		text: CRMText;
		onEdit: (contactID: string) => void;
		onCreate: (organizationID: string) => void;
		canCreate?: boolean;
	};

	let { organizationID, contacts, text, onEdit, onCreate, canCreate = true }: Props = $props();
	let organizationContacts = $derived(contacts.filter((contact) => contact.organizationID === organizationID));
</script>

<Field.Field>
	<div class="flex items-center justify-between gap-3">
		<Field.Label>{text.externalContact}</Field.Label>
		<Button disabled={!canCreate} type="button" variant="outline" size="sm" onclick={() => onCreate(organizationID)}>{text.newExternalContact}</Button>
	</div>
	{#if organizationContacts.length === 0}
		<Empty.Root class="p-3"><Empty.Header><Empty.Title>{text.noOrganizationContacts}</Empty.Title></Empty.Header></Empty.Root>
	{:else}
		<div class="grid gap-2">
			{#each organizationContacts as contact (contact.id)}
				<Button
					type="button"
					variant="outline"
					class="h-auto min-w-0 justify-start px-3 py-2 text-left"
					aria-label={`${text.editContact} · ${contact.name}`}
					onclick={() => onEdit(contact.id)}
				>
					<span class="min-w-0 flex-1">
						<span class="block truncate">{contact.name}</span>
						<span class="block truncate text-xs text-muted-foreground">{contact.email || contact.phone}</span>
					</span>
				</Button>
			{/each}
		</div>
	{/if}
</Field.Field>
