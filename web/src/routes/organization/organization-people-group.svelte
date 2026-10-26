<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { toast } from 'svelte-sonner';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import UserIcon from '@lucide/svelte/icons/user';
	import CountBadge from '$lib/components/count-badge.svelte';
	import MemberCard from '$lib/components/member/member-card.svelte';
	import MemberCell from '$lib/components/member/member-cell.svelte';
	import { memberRolesOf, type MemberProfile } from '$lib/components/member/member-role';
	import { memberText } from '$lib/components/member/member-text';
	import * as Item from '$lib/components/ui/item';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { openDirectMessageWith } from '$lib/messenger/open-direct-message';
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
	const words = createPageText(memberText);

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

	function tenureOf(hireDate: string | undefined): { label: string; detail: string } | undefined {
		const tenure = organizationTenure(hireDate, today);
		if (!tenure) return undefined;
		const label = tenure.years ? `${tenure.years}${text.tenureYearUnit}+` : `${tenure.months}${text.tenureMonthUnit}`;
		return { label, detail: `${hireDate} · ${tenure.years}${text.tenureYearUnit} ${tenure.months}${text.tenureMonthUnit}` };
	}

	function memberOf(record: UserRecord): MemberProfile {
		return {
			memberID: record.memberID,
			name: record.name || record.email,
			email: record.email,
			image: record.image,
			jobTitle: record.jobTitle || text.noTitle,
			phoneNumber: record.phoneNumber,
			roles: memberRolesOf(record.memberID, section.representativeID, section.responsibleUserID)
		};
	}

	async function messageMember(member: MemberProfile) {
		const opening = await openDirectMessageWith(member, page.url.pathname);
		if (opening === 'no-messenger-account') toast.error(words.noMessengerAccount.replace('{name}', member.name));
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
			{@const member = memberOf(record)}
			{#if isMobile.current}
				<MemberCell
					{member}
					isSelected={selectedUserID === record.memberID}
					onSelect={() => selectRecord(record)}
					onMessage={() => void messageMember(member)}
					testID={`organization-person-card-${record.memberID}`}
					selectTestID={`organization-person-node-${record.memberID}`}
				/>
			{:else}
				<MemberCard
					{member}
					tenure={tenureOf(record.hireDate)}
					isSelected={selectedUserID === record.memberID}
					onSelect={() => selectRecord(record)}
					onMessage={() => void messageMember(member)}
					testID={`organization-person-card-${record.memberID}`}
					selectTestID={`organization-person-node-${record.memberID}`}
				/>
			{/if}
		{/each}
	</Item.Group>
	{#each node.children as childNode (childNode.section.id)}
		<OrganizationPeopleGroup node={childNode} {selectedUserID} {text} {selectRecord} />
	{/each}
</section>
