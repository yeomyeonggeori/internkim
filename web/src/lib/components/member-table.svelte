<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import * as Empty from '$lib/components/ui/empty';
	import type { Snippet } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';

	export type TabledMember = {
		id: string;
		name: string;
		email: string;
		image?: string;
		hireDate?: string;
		role: string;
	};

	export type MemberTableText = {
		name: string;
		email: string;
		hireDate: string;
		role: string;
		empty: string;
	};

	type Props = {
		members: TabledMember[];
		isLoading?: boolean;
		hasLoadError?: boolean;
		text: MemberTableText;
		minimumWidth?: string;
		extraHeaders?: Snippet;
		extraCells?: Snippet<[TabledMember]>;
		mobileExtra?: Snippet<[TabledMember]>;
		extraColumnCount?: number;
	};

	let {
		members,
		isLoading = false,
		hasLoadError = false,
		text,
		minimumWidth = 'min-w-[560px]',
		extraHeaders,
		extraCells,
		mobileExtra,
		extraColumnCount = 0
	}: Props = $props();

	const headClass = 'h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground';
	const isMobile = new MediaQuery('(max-width: 639px)');
</script>

{#if !hasLoadError || members.length > 0 || isLoading}
{#if isLoading && members.length === 0}
	<div role="status" aria-label={text.name} aria-busy="true" class="overflow-hidden rounded-lg border" data-testid="member-list-loading-skeleton"><div aria-hidden="true" class="divide-y">{#each [0, 1, 2, 3] as row (row)}<div class="flex items-center gap-3 p-3"><Skeleton class="size-10 shrink-0 rounded-full sm:size-7" /><div class="grid flex-1 gap-2 sm:grid-cols-[1fr_1.5fr_1fr_5rem]"><Skeleton class="h-4 w-24" /><Skeleton class="h-4 w-4/5" /><Skeleton class="hidden h-4 w-20 sm:block" /><Skeleton class="h-5 w-16 rounded-full" /></div>{#if extraColumnCount > 0}<Skeleton class="hidden h-8 w-12 sm:block" />{/if}</div>{/each}</div></div>
{:else if isMobile.current}
	<ul class="divide-y rounded-lg border" aria-label={text.name}>
		{#each members as member (member.id)}
			<li class="grid min-w-0 gap-3 p-3">
				<div class="flex min-w-0 items-start gap-3">
					<PersonAvatar name={displayPersonName(member.name)} email={member.email} image={member.image ?? ''} class="size-10 shrink-0" />
					<div class="min-w-0 flex-1">
						<p class="break-words font-medium">{displayPersonName(member.name)}</p>
						<p class="break-all text-sm text-muted-foreground">{member.email || '-'}</p>
						<div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
							<Badge variant={member.role === 'admin' ? 'secondary' : 'outline'}>{member.role}</Badge>
							<span>{text.hireDate}: {member.hireDate || '-'}</span>
						</div>
					</div>
				</div>
				{@render mobileExtra?.(member)}
			</li>
		{:else}
			<li><Empty.Root><Empty.Header><Empty.Title>{text.empty}</Empty.Title></Empty.Header></Empty.Root></li>
		{/each}
	</ul>
{:else}
<div class="overflow-x-auto rounded-lg border">
	<Table.Root class={minimumWidth}>
		<Table.Header class="bg-muted/40">
			<Table.Row class="hover:bg-transparent">
				<Table.Head class={headClass}>{text.name}</Table.Head>
				<Table.Head class={headClass}>{text.email}</Table.Head>
				<Table.Head class={headClass}>{text.hireDate}</Table.Head>
				<Table.Head class={headClass}>{text.role}</Table.Head>
				{@render extraHeaders?.()}
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#each members as member (member.id)}
				<Table.Row>
					<Table.Cell class="font-medium">
						<div class="flex items-center gap-2">
							<PersonAvatar name={displayPersonName(member.name)} email={member.email} image={member.image ?? ''} class="size-7" />
							{displayPersonName(member.name)}
						</div>
					</Table.Cell>
					<Table.Cell class="text-muted-foreground">{member.email || '-'}</Table.Cell>
					<Table.Cell class="text-muted-foreground">{member.hireDate || '-'}</Table.Cell>
					<Table.Cell>
						<Badge variant={member.role === 'admin' ? 'secondary' : 'outline'}>{member.role}</Badge>
					</Table.Cell>
					{@render extraCells?.(member)}
				</Table.Row>
			{/each}
			{#if members.length === 0}
				<Table.Row class="hover:bg-transparent">
					<Table.Cell colspan={4 + extraColumnCount} class="whitespace-normal p-0">
						<Empty.Root><Empty.Header><Empty.Title>{text.empty}</Empty.Title></Empty.Header></Empty.Root>
					</Table.Cell>
				</Table.Row>
			{/if}
		</Table.Body>
	</Table.Root>
</div>
{/if}
{/if}
