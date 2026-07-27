<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import CrownIcon from '@lucide/svelte/icons/crown';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import FlagTriangleRightIcon from '@lucide/svelte/icons/flag-triangle-right';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import UserIcon from '@lucide/svelte/icons/user';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import { Badge } from '$lib/components/ui/badge';
	import OrganizationCountBadge from './organization-count-badge.svelte';
	import OrganizationPersonContactActions from './organization-person-contact-actions.svelte';
	import * as Item from '$lib/components/ui/item';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import type { UserRecord } from '$lib/organization/types';
	import { organizationTenure } from './organization-tenure';
	import OrganizationPeopleGroup from './organization-people-group.svelte';
	import type { OrganizationSectionNode } from './organization-section-tree';
	import type { organizationDirectoryText } from './text';

	let {
		node,
		selectedUserID,
		text,
		selectRecord
	}: {
		node: OrganizationSectionNode;
		selectedUserID: string;
		text: typeof organizationDirectoryText.ko;
		selectRecord: (record: UserRecord) => void;
	} = $props();

	const section = $derived(node.section);
	let headerElement = $state<HTMLElement>();
	let isHeaderStuck = $state(false);
	const depth = $derived(Math.min(section.depth, 8));
	const today = new Date();

	$effect(() => {
		const header = headerElement;
		if (!header) return;
		const scrollElement = header.closest('[data-organization-scroll]');
		if (!scrollElement) return;
		const stickyOffset = depth * 36;
		const updateStuckState = () => {
			isHeaderStuck = header.getBoundingClientRect().top - scrollElement.getBoundingClientRect().top <= stickyOffset + 0.5;
		};
		updateStuckState();
		scrollElement.addEventListener('scroll', updateStuckState, { passive: true });
		return () => scrollElement.removeEventListener('scroll', updateStuckState);
	});

	function tenureLabel(hireDate: string | undefined): string {
		const tenure = organizationTenure(hireDate, today);
		if (!tenure) return '';
		if (!tenure.years) return `${tenure.months}${text.tenureMonthUnit}`;
		return `${tenure.years}${text.tenureYearUnit}+`;
	}

	function tenureDetail(hireDate: string | undefined): string {
		const tenure = organizationTenure(hireDate, today);
		if (!tenure) return '';
		return `${hireDate} · ${tenure.years}${text.tenureYearUnit} ${tenure.months}${text.tenureMonthUnit}`;
	}

	function personLabel(record: UserRecord): string {
		return record.name || record.email;
	}

	function responsibility(record: UserRecord): { isCompanyWide: boolean; label: string; class: string } | undefined {
		if (record.userID === section.companyResponsibleUserID)
			return { isCompanyWide: true, label: text.companyRepresentative, class: 'bg-blue-500 text-white dark:bg-blue-600' };
		if (record.userID === section.responsibleUserID)
			return { isCompanyWide: false, label: text.responsible, class: 'bg-emerald-500 text-white dark:bg-emerald-600' };
		return undefined;
	}
</script>

<section class="relative min-w-0 pb-3" data-testid={`organization-section-${section.id || 'root'}`}>
	{#if section.records.length > 0 || node.children.length > 0}
		<span class="bg-border absolute bottom-0 top-9 w-px" style={`left: ${depth * 16 + 20}px; z-index: ${39 - depth * 2}`} aria-hidden="true"></span>
	{/if}
	<div
		class="bg-background sticky flex h-9 items-center gap-1 pr-2"
		style={`padding-left: ${depth * 16 + 8}px; top: ${depth * 36}px; z-index: ${40 - depth * 2}`}
		bind:this={headerElement}
	>
		<span class="text-muted-foreground grid size-6 shrink-0 place-items-center">
			{#if section.id}
				<ComponentIcon class="size-4" />
			{:else}
				<Building2Icon class="size-4" />
			{/if}
		</span>
		<h3 class="min-w-0 truncate text-sm font-medium">{section.name}</h3>
		<OrganizationCountBadge count={section.memberCount} label={`${section.memberCount}${text.memberCountUnit}`}>
			{#snippet icon()}
				<UserIcon class="size-3" />
			{/snippet}
		</OrganizationCountBadge>
		<span
			class={[
				'from-foreground/12 pointer-events-none absolute top-full right-0 h-2 bg-gradient-to-b to-transparent transition-opacity duration-200 [mask-image:linear-gradient(to_right,transparent,black_12%,black_88%,transparent)]',
				isHeaderStuck ? 'opacity-100' : 'opacity-0'
			]}
			style={`left: ${depth * 16 + 8}px`}
			aria-hidden="true"
		></span>
	</div>
	<Item.Group
		class="grid grid-cols-[repeat(auto-fill,minmax(11rem,1fr))] gap-2 pr-2"
		style={`padding-left: ${depth * 16 + 36}px`}
		data-testid={`organization-members-${section.id || 'root'}`}
	>
		{#each section.records as record (record.userID)}
			{@const leadership = responsibility(record)}
			<Item.Root
				variant="outline"
				class={[
					'bg-card hover:bg-accent/40 relative flex-col items-center gap-2 px-4 pt-7 pb-2 text-center',
					selectedUserID === record.userID && 'border-primary bg-accent/40'
				]}
			>
				<div class="absolute top-2 left-2 z-10 grid justify-items-start gap-1">
					{#if leadership}
						<Badge variant="secondary" class={leadership.class}>
							{#if leadership.isCompanyWide}
								<CrownIcon />
							{:else}
								<FlagTriangleRightIcon />
							{/if}
							{leadership.label}
						</Badge>
					{/if}
					{#if tenureLabel(record.hireDate)}
						<Tooltip.Root>
							<Tooltip.Trigger>
								{#snippet child({ props })}
									<Badge {...props} variant="outline" class="bg-background h-5 gap-1 rounded-full px-1.5 font-normal tabular-nums">
										<TimerIcon class="size-3" />
										{tenureLabel(record.hireDate)}
									</Badge>
								{/snippet}
							</Tooltip.Trigger>
							<Tooltip.Content side="top">{tenureDetail(record.hireDate)}</Tooltip.Content>
						</Tooltip.Root>
					{/if}
				</div>
				<button
					type="button"
					class="grid w-full justify-items-center gap-2"
					onclick={() => selectRecord(record)}
					data-testid={`organization-person-node-${record.userID}`}
				>
					<PersonAvatar name={record.name} email={record.email} seed={record.userID} image={record.image ?? ''} class="size-20" />
					<span class="grid w-full gap-0.5 text-center">
						<span class="truncate text-sm leading-snug font-medium">{personLabel(record)}</span>
						<span class="text-muted-foreground truncate text-xs leading-normal">{record.jobTitle || text.noTitle}</span>
					</span>
				</button>
				<OrganizationPersonContactActions email={record.email} phoneNumber={record.phoneNumber ?? ''} {text} />
			</Item.Root>
		{/each}
	</Item.Group>
	{#each node.children as childNode (childNode.section.id)}
		<OrganizationPeopleGroup node={childNode} {selectedUserID} {text} {selectRecord} />
	{/each}
</section>
