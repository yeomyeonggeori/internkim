<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import type { OrgchartOrganizationMemberNode, OrgchartOrganizationSection } from './orgchart-organization-model';
	import type { UserRecord } from './orgchart-types';
	import type { orgchartDirectoryText } from './text';

	type OrgchartOrganizationCardProps = {
		section: OrgchartOrganizationSection;
		selectedUserID: string;
		text: typeof orgchartDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	};

	let {
		section,
		selectedUserID,
		text,
		selectRecord
	}: OrgchartOrganizationCardProps = $props();

	const rowPreviewLimit = 5;
	let isExpanded = $state(false);

	const organizationRows = $derived([section.leader, ...flattenMemberNodes(section.memberNodes)]);
	const shouldShowAllRows = $derived(isExpanded || organizationRows.some((record, index) => index >= rowPreviewLimit && record.userID === selectedUserID));
	const visibleRows = $derived(shouldShowAllRows ? organizationRows : organizationRows.slice(0, rowPreviewLimit));
	const hiddenRowCount = $derived(Math.max(organizationRows.length - visibleRows.length, 0));

	function personLabel(record: UserRecord): string {
		return record.name || record.email;
	}

	function personTitle(record: UserRecord): string {
		return record.jobTitle || text.noTitle;
	}

	function rowClass(record: UserRecord): string {
		return [
			'grid w-full min-w-0 grid-cols-[minmax(0,1.25fr)_minmax(96px,0.75fr)] items-center gap-3 rounded-md px-3 py-3 text-left text-sm transition hover:bg-muted/40',
			selectedUserID === record.userID && 'bg-primary/5 ring-1 ring-primary'
		].filter(Boolean).join(' ');
	}

	function flattenMemberNodes(memberNodes: OrgchartOrganizationMemberNode[]): UserRecord[] {
		return memberNodes.flatMap((memberNode) => [memberNode.record, ...flattenMemberNodes(memberNode.children)]);
	}
</script>

<div class="min-w-0" data-testid={`orgchart-organization-section-${section.id}`}>
	<Card.Root class="gap-0 overflow-hidden border border-border bg-card py-0 shadow-sm ring-0" data-testid={`orgchart-team-column-${section.id}`}>
		<Card.Header class="border-b px-4 py-4">
			<div class="grid gap-3">
				<div class="flex items-center justify-between gap-3">
					<div class="flex min-w-0 items-center gap-2">
						<NetworkIcon class="size-5 shrink-0 text-primary" />
						<Card.Title class="truncate text-lg">{section.name}</Card.Title>
						<Card.Description class="shrink-0 text-base">
							{section.memberCount}{text.memberCountUnit}
						</Card.Description>
					</div>
					{#if section.isUnassigned}
						<Badge variant="outline" class="shrink-0">{text.unassignedTeam}</Badge>
					{/if}
				</div>
				<div class="flex min-w-0 items-center gap-3 text-sm">
					<span class="shrink-0 font-medium text-muted-foreground">{text.teamLead}</span>
					<span class="truncate font-semibold">{personLabel(section.leader)}</span>
					<span class="truncate text-muted-foreground">{personTitle(section.leader)}</span>
				</div>
			</div>
		</Card.Header>
		<Card.Content class="p-3" data-testid={`orgchart-organization-card-${section.id}`}>
			<div class="grid" data-testid={`orgchart-organization-members-${section.id}`}>
				{#each visibleRows as record, index (record.userID)}
					<div class="border-b py-1 last:border-b-0" data-testid={index === 0 ? undefined : `orgchart-tree-node-${record.userID}`}>
						{@render personRow(record)}
					</div>
				{/each}
			</div>

			{#if hiddenRowCount > 0}
				<Button type="button" variant="ghost" size="sm" class="mt-2 justify-start px-2 text-muted-foreground" onclick={() => (isExpanded = true)}>
					{ text.moreMembersPrefix } {hiddenRowCount}{ text.moreMembersSuffix }
				</Button>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

{#snippet personRow(record: UserRecord)}
	<button type="button" class={rowClass(record)} onclick={() => selectRecord(record)} data-testid={`orgchart-person-node-${record.userID}`}>
		<span class="flex min-w-0 items-center gap-3">
			<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-8 shrink-0" />
			<span class="truncate font-semibold">{personLabel(record)}</span>
		</span>
		<span class="truncate text-muted-foreground">{personTitle(record)}</span>
	</button>
{/snippet}
