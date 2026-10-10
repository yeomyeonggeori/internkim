<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { displayPersonName } from '$lib/person-name.svelte';
	import CrownIcon from '@lucide/svelte/icons/crown';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import FlagTriangleRightIcon from '@lucide/svelte/icons/flag-triangle-right';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import UserIcon from '@lucide/svelte/icons/user';
	import { Badge } from '$lib/components/ui/badge';
	import CountBadge from '$lib/components/count-badge.svelte';
	import OrganizationPersonContactActions from './organization-person-contact-actions.svelte';
	import * as Item from '$lib/components/ui/item';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import type { UserRecord } from '$lib/organization/types';
	import { organizationSectionTrailingGap } from './organization-section-tree';
	import { organizationTenure } from './organization-tenure';
	import OrganizationPeopleGroup from './organization-people-group.svelte';
	import type { OrganizationSectionNode } from './organization-section-tree';
	import type { organizationDirectoryText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	let {
		node,
		selectedUserID,
		text,
		selectRecord
	}: {
		node: OrganizationSectionNode;
		selectedUserID: string;
		text: PageText<typeof organizationDirectoryText>;
		selectRecord: (record: UserRecord) => void;
	} = $props();

	const section = $derived(node.section);
	let sectionElement = $state<HTMLElement>();
	let headerElement = $state<HTMLElement>();
	let isHeaderStuck = $state(false);
	let isNestedHeaderStuck = $state(false);
	const isMobile = new MediaQuery('(max-width: 639px)');
	const depth = $derived(Math.min(section.depth, isMobile.current ? 2 : 8));
	const railBottom = $derived(organizationSectionTrailingGap(node, 12));
	const today = new Date();

	$effect(() => {
		const header = headerElement;
		const sectionBox = sectionElement;
		if (!header || !sectionBox) return;
		const scrollElement = header.closest('[data-organization-scroll]');
		if (!scrollElement) return;
		const stickyOffset = depth * 36;
		const isPinned = (element: HTMLElement, offset: number) => {
			const distance = element.getBoundingClientRect().top - scrollElement.getBoundingClientRect().top;
			return Math.abs(distance - offset) <= 0.5;
		};
		const updateStuckState = () => {
			isHeaderStuck = isPinned(header, stickyOffset);
			isNestedHeaderStuck = Array.from(sectionBox.querySelectorAll<HTMLElement>('[data-organization-header]')).some(
				(nestedHeader) => nestedHeader !== header && isPinned(nestedHeader, Number(nestedHeader.dataset.stickyOffset ?? 0))
			);
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
		return displayPersonName(record.name || record.email);
	}

	function responsibility(record: UserRecord): { isCompanyWide: boolean; label: string; class: string } | undefined {
		if (record.memberID === section.companyResponsibleUserID)
			return { isCompanyWide: true, label: text.companyRepresentative, class: 'bg-blue-500 text-white dark:bg-blue-600' };
		if (record.memberID === section.responsibleUserID)
			return { isCompanyWide: false, label: text.responsible, class: 'bg-emerald-500 text-white dark:bg-emerald-600' };
		return undefined;
	}
</script>

<section
	class={['relative min-w-0 pb-3', depth > 0 && 'mt-4']}
	bind:this={sectionElement}
	data-testid={`organization-section-${section.id || 'root'}`}
>
	{#if section.records.length > 0 || node.children.length > 0}
		<span
			class="bg-border absolute top-9 w-px"
			style={`left: ${depth * 16 + 20}px; bottom: ${railBottom}px; z-index: ${39 - depth * 2}`}
			aria-hidden="true"
		></span>
	{/if}
	<div
		class="bg-background sticky flex h-9 items-center gap-1 pr-2"
		style={`padding-left: ${depth * 16 + 8}px; top: ${depth * 36}px; z-index: ${40 - depth * 2}`}
		data-organization-header
		data-sticky-offset={depth * 36}
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
		{#if isMobile.current}
			<span class="text-muted-foreground ml-1 shrink-0 text-xs tabular-nums" aria-label={`${section.memberCount}${text.memberCountUnit}`}>{section.memberCount}</span>
		{:else}
			<CountBadge count={section.memberCount} label={`${section.memberCount}${text.memberCountUnit}`}>
				{#snippet icon()}
					<UserIcon class="size-3" />
				{/snippet}
			</CountBadge>
		{/if}
		<span
			class={[
				'from-foreground/12 pointer-events-none absolute top-full right-0 h-2 bg-gradient-to-b to-transparent transition-opacity duration-200 [mask-image:linear-gradient(to_right,transparent,black_12%,black_88%,transparent)]',
				isHeaderStuck && !isNestedHeaderStuck ? 'opacity-100' : 'opacity-0'
			]}
			style={`left: ${depth * 16 + 8}px`}
			aria-hidden="true"
		></span>
	</div>
	<Item.Group
		class="grid grid-cols-1 gap-2 pr-2 max-sm:gap-0 sm:grid-cols-[repeat(auto-fill,minmax(11rem,1fr))]"
		style={`padding-left: ${depth * 16 + 36}px`}
		data-testid={`organization-members-${section.id || 'root'}`}
	>
		{#each section.records as record (record.memberID)}
			{@const leadership = responsibility(record)}
			{@const tenure = tenureLabel(record.hireDate)}
			<Item.Root
				variant={isMobile.current ? 'default' : 'outline'}
				class={[
					'bg-card hover:bg-accent/40 relative gap-2 max-sm:flex-nowrap max-sm:px-2 max-sm:py-1.5 sm:flex-col sm:items-center sm:px-4 sm:pt-7 sm:pb-2 sm:text-center',
					selectedUserID === record.memberID && 'shadow-lg max-sm:bg-accent max-sm:shadow-none'
				]}
				data-testid={`organization-person-card-${record.memberID}`}
			>
				{#if !isMobile.current}
					<div class="absolute top-2 left-2 z-10 grid items-center justify-items-start gap-1">
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
						{#if tenure}
							<Tooltip.Root>
								<Tooltip.Trigger>
									{#snippet child({ props })}
										<Badge {...props} variant="outline" class="bg-background h-5 gap-1 rounded-full px-1.5 font-normal tabular-nums">
											<TimerIcon class="size-3" />
											{tenure}
										</Badge>
									{/snippet}
								</Tooltip.Trigger>
								<Tooltip.Content side="top">{tenureDetail(record.hireDate)}</Tooltip.Content>
							</Tooltip.Root>
						{/if}
					</div>
				{/if}
				<button
					type="button"
					class="flex min-h-11 w-full min-w-0 items-center gap-3 max-sm:flex-1 sm:grid sm:justify-items-center sm:gap-2"
					onclick={() => selectRecord(record)}
					data-testid={`organization-person-node-${record.memberID}`}
				>
					<PersonAvatar name={displayPersonName(record.name)} email={record.email} seed={record.memberID} image={record.image ?? ''} class="size-10 shrink-0 max-sm:size-9 sm:size-20" />
					{#if isMobile.current}
						<span class="grid min-w-0 w-full gap-0.5 text-left">
							<span class="flex min-w-0 items-center gap-2">
								<span class="truncate text-sm leading-snug font-medium">{personLabel(record)}</span>
								{#if leadership}
									<Badge variant="secondary" class="font-normal">
										{#if leadership.isCompanyWide}<CrownIcon />{:else}<FlagTriangleRightIcon />{/if}
										{leadership.label}
									</Badge>
								{/if}
							</span>
							<span class="text-muted-foreground flex min-w-0 items-center gap-2 text-xs leading-normal">
								<span class="truncate">{record.jobTitle || text.noTitle}</span>
								{#if tenure}
									<span class="inline-flex shrink-0 items-center gap-1 tabular-nums" title={tenureDetail(record.hireDate)}><TimerIcon class="size-3" />{tenure}</span>
								{/if}
							</span>
						</span>
					{:else}
						<span class="grid min-w-0 w-full gap-0.5 text-left sm:text-center">
							<span class="break-words text-sm leading-snug font-medium sm:truncate">{personLabel(record)}</span>
							<span class="text-muted-foreground break-words text-xs leading-normal sm:truncate">{record.jobTitle || text.noTitle}</span>
						</span>
					{/if}
				</button>
				<OrganizationPersonContactActions email={record.email} phoneNumber={record.phoneNumber ?? ''} {text} />
			</Item.Root>
		{/each}
	</Item.Group>
	{#each node.children as childNode (childNode.section.id)}
		<OrganizationPeopleGroup node={childNode} {selectedUserID} {text} {selectRecord} />
	{/each}
</section>
