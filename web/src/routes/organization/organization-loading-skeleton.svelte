<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import * as Item from '$lib/components/ui/item';
	import MemberCardSkeleton from '$lib/components/member/member-card-skeleton.svelte';
	import MemberCellSkeleton from '$lib/components/member/member-cell-skeleton.svelte';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import type { PageText } from '$lib/i18n/page-text.svelte';
	import type { organizationDirectoryText } from './text';

	let { organizationCount = 5, personCount = 8, label, text, canManage = false }: { organizationCount?: number; personCount?: number; label: string; text: PageText<typeof organizationDirectoryText>; canManage?: boolean } = $props();
	const isPhone = new IsMobile();
</script>

<div class="grid h-full min-h-0 lg:grid-cols-[270px_minmax(0,1fr)]" data-testid="organization-skeleton" role="status" aria-label={label} aria-busy="true">
	<div aria-hidden="true" class="hidden min-h-0 border-r px-2 pt-3 pb-4 lg:block">
		<div class="flex h-9 items-center gap-1 pl-2"><span class="text-muted-foreground grid size-6 shrink-0 place-items-center"><Building2Icon class="size-4" /></span><Skeleton class="h-4 w-20" /></div>
		{#each Array.from({ length: organizationCount }) as _, organizationIndex (organizationIndex)}
			<div class="relative flex h-9 items-center gap-1 pl-6 pr-2"><span class="absolute inset-y-0 left-5 w-px bg-border"></span><Skeleton class="size-6 shrink-0" /><Skeleton class="h-4" style="width: {45 + ((organizationIndex * 13) % 30)}%" /></div>
		{/each}
	</div>
	<div class="grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
		<div class="flex min-w-0 items-center gap-2 px-4 py-3 sm:px-6">
			{#if canManage}<Button disabled size="icon" variant="outline" aria-label={text.inviteMember}><UserPlusIcon /></Button>{/if}
			<Button disabled size="icon" variant="outline" aria-label={text.openOrganizations} class="lg:hidden"><NetworkIcon /></Button>
			<FilterCombobox options={[]} label={text.selectOrganization} disabled class="min-w-0 flex-1 max-sm:hidden sm:ml-auto sm:w-52 sm:flex-none" />
			<FilterCombobox options={[]} label={text.selectEmployee} disabled class="order-first w-full min-w-0 max-sm:w-auto max-sm:flex-1 sm:order-none sm:w-72 sm:flex-none" />
		</div>
		<div aria-hidden="true" class="min-h-0 overflow-hidden px-4 sm:px-6">
			<div class="grid pt-4 pb-6 max-sm:pt-1">
				<section class="relative min-w-0 pb-3">
					<span class="absolute top-9 bottom-3 left-5 w-px bg-border"></span>
					<div class="flex h-9 items-center gap-1 pr-2 pl-2"><span class="text-muted-foreground grid size-6 shrink-0 place-items-center"><Building2Icon class="size-4" /></span><Skeleton class="h-4 w-16" /><Skeleton class="h-5 w-9 rounded-full max-sm:ml-1 max-sm:h-3 max-sm:w-3 max-sm:rounded-sm" /></div>
					<section class="relative mt-4 min-w-0 pb-3">
						<span class="absolute top-9 bottom-3 left-9 w-px bg-border"></span>
						<div class="flex h-9 items-center gap-1 pr-2 pl-6"><span class="text-muted-foreground grid size-6 shrink-0 place-items-center"><ComponentIcon class="size-4" /></span><Skeleton class="h-4 w-24" /><Skeleton class="h-5 w-9 rounded-full max-sm:ml-1 max-sm:h-3 max-sm:w-3 max-sm:rounded-sm" /></div>
						<Item.Group class="grid grid-cols-1 gap-2 pr-2 pl-[52px] max-sm:gap-0 sm:grid-cols-[repeat(auto-fill,minmax(11rem,1fr))]">
							{#each Array.from({ length: personCount }) as _, personIndex (personIndex)}
								{#if isPhone.current}<MemberCellSkeleton />{:else}<MemberCardSkeleton />{/if}
							{/each}
						</Item.Group>
					</section>
				</section>
			</div>
		</div>
	</div>
</div>
