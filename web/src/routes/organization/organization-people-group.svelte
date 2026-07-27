<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { Badge } from '$lib/components/ui/badge';
	import * as Item from '$lib/components/ui/item';
	import type { UserRecord } from '$lib/organization/types';
	import type { OrganizationOrganizationSection } from './organization-model';
	import type { organizationDirectoryText } from './text';

	let {
		section,
		selectedUserID,
		text,
		selectRecord
	}: {
		section: OrganizationOrganizationSection;
		selectedUserID: string;
		text: typeof organizationDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	} = $props();

	const depth = $derived(Math.min(section.depth, 8));

	function personLabel(record: UserRecord): string {
		return record.name || record.email;
	}

	function responsibilityLabel(record: UserRecord): string {
		if (record.userID === section.companyResponsibleUserID) return text.companyResponsible;
		if (record.userID === section.responsibleUserID) return text.organizationResponsible;
		return '';
	}
</script>

<section class="relative min-w-0 pb-3" data-testid={`organization-section-${section.id || 'root'}`}>
	{#each { length: depth } as _, level (level)}
		<span class="bg-border absolute inset-y-0 w-px" style={`left: ${level * 16 + 20}px`} aria-hidden="true"></span>
	{/each}
	{#if section.records.length > 0}
		<span class="bg-border absolute bottom-0 top-9 w-px" style={`left: ${depth * 16 + 20}px`} aria-hidden="true"></span>
	{/if}
	<div class="flex h-9 items-center gap-1 pr-2" style={`padding-left: ${depth * 16 + 8}px`}>
		<span class="text-muted-foreground grid size-6 shrink-0 place-items-center">
			{#if section.id}
				<UsersRoundIcon class="size-4" />
			{:else}
				<Building2Icon class="size-4" />
			{/if}
		</span>
		<h3 class="min-w-0 flex-1 truncate text-sm font-medium">{section.name}</h3>
		<span class="text-muted-foreground shrink-0 text-xs tabular-nums">{section.memberCount}{text.memberCountUnit}</span>
	</div>
	<Item.Group class="gap-2 pr-2" style={`padding-left: ${depth * 16 + 36}px`} data-testid={`organization-members-${section.id || 'root'}`}>
		{#each section.records as record (record.userID)}
			{@const responsibility = responsibilityLabel(record)}
			<Item.Root variant="outline" class={selectedUserID === record.userID ? 'ring-ring ring-1' : ''}>
				{#snippet child({ props })}
					<button
						{...props}
						type="button"
						onclick={() => selectRecord(record)}
						data-testid={`organization-person-node-${record.userID}`}
					>
						<Item.Media>
							<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-9" />
						</Item.Media>
						<Item.Content>
							<Item.Title>{personLabel(record)}</Item.Title>
							<Item.Description>{record.jobTitle || text.noTitle}</Item.Description>
						</Item.Content>
						{#if responsibility}
							<Item.Actions>
								<Badge variant="secondary">{responsibility}</Badge>
							</Item.Actions>
						{/if}
					</button>
				{/snippet}
			</Item.Root>
		{/each}
	</Item.Group>
</section>
