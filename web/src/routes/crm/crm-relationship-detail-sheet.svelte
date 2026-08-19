<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import { crmLabel } from './crm-labels';
	import type { CRMOrganization, CRMActivity, CRMContact, CRMOpportunity } from './crm-types';
	import CRMRelationshipDetailView from './crm-relationship-detail-view.svelte';
	import { getStatusVariant } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		organization: CRMOrganization | undefined;
		contacts: CRMContact[];
		opportunities: CRMOpportunity[];
		activities: CRMActivity[];
		text: CRMText;
		onEdit: (organizationID: string) => void;
	};
	let { open = $bindable(false), organization, contacts, opportunities, activities, text, onEdit }: Props = $props();
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="w-full overflow-y-auto scroll-pb-24 sm:max-w-xl">
		{#if organization}
			<Sheet.Header class="border-b pb-4">
				<div class="flex items-start justify-between gap-3 pr-8">
					<div class="flex flex-wrap items-center gap-2">{#each organization.types as organizationType (organizationType)}<Badge variant="outline">{crmLabel(text.organizationTypes, organizationType)}</Badge>{/each}<Badge variant={getStatusVariant(organization.status)}>{text.organizationStatuses[organization.status]}</Badge></div>
					<Button type="button" variant="outline" size="sm" onclick={() => onEdit(organization.id)}><PencilIcon data-icon="inline-start" />{text.edit}</Button>
				</div>
				<Sheet.Title>{organization.name}</Sheet.Title>
				<Sheet.Description>{organization.description}</Sheet.Description>
			</Sheet.Header>
				<div class="px-4 pb-24 pt-4"><CRMRelationshipDetailView {organization} {contacts} {opportunities} {activities} {text} /></div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
