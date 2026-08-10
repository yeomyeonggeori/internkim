<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import type { CRMAccount, CRMActivity, CRMContact, CRMOpportunity } from './crm-types';
	import CRMRelationshipDetailView from './crm-relationship-detail-view.svelte';
	import { getStatusVariant } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		account: CRMAccount | undefined;
		contacts: CRMContact[];
		opportunities: CRMOpportunity[];
		activities: CRMActivity[];
		text: CRMText;
		onEdit: (accountID: string) => void;
	};
	let { open = $bindable(false), account, contacts, opportunities, activities, text, onEdit }: Props = $props();
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="w-full overflow-y-auto scroll-pb-24 sm:max-w-xl">
		{#if account}
			<Sheet.Header class="border-b pb-4">
				<div class="flex items-start justify-between gap-3 pr-8">
					<div class="flex flex-wrap items-center gap-2">{#each account.types as accountType (accountType)}<Badge variant="outline">{text.accountTypes[accountType]}</Badge>{/each}<Badge variant={getStatusVariant(account.status)}>{text.accountStatuses[account.status]}</Badge></div>
					<Button type="button" variant="outline" size="sm" onclick={() => onEdit(account.id)}><PencilIcon data-icon="inline-start" />{text.edit}</Button>
				</div>
				<Sheet.Title>{account.name}</Sheet.Title>
				<Sheet.Description>{account.description}</Sheet.Description>
			</Sheet.Header>
				<div class="px-4 pb-24 pt-4"><CRMRelationshipDetailView {account} {contacts} {opportunities} {activities} {text} /></div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
