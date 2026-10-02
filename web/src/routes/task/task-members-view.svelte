<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import MemberTable, { type MemberTableText } from '$lib/components/member-table.svelte';
	import type { TaskMember } from './task-types';

	type MembersText = MemberTableText & {
		title: string;
		active: string;
		completed: string;
		distance: string;
	};

	type Props = {
		members: TaskMember[];
		text: MembersText;
	};

	let { members, text }: Props = $props();

	const headClass = 'h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground';
	const countsByID = $derived(new Map(members.map((member) => [member.id, member])));
</script>

{#snippet workHeaders()}
	<Table.Head class={headClass}>{text.active}</Table.Head>
	<Table.Head class={headClass}>{text.completed}</Table.Head>
	<Table.Head class={headClass}>{text.distance}</Table.Head>
{/snippet}

{#snippet workCells(member: { id: string })}
	{@const counted = countsByID.get(member.id)}
	<Table.Cell class="text-right tabular-nums">{counted?.activeTaskCount ?? 0}</Table.Cell>
	<Table.Cell class="text-right tabular-nums">{counted?.completeTaskCount ?? 0}</Table.Cell>
	<Table.Cell class="text-right font-medium tabular-nums">{counted?.distance ?? counted?.score ?? 0}</Table.Cell>
{/snippet}

{#snippet mobileWork(member: { id: string })}
	{@const counted = countsByID.get(member.id)}
	<dl class="grid grid-cols-3 gap-2 text-sm">
		<div><dt class="text-xs text-muted-foreground">{text.active}</dt><dd class="tabular-nums">{counted?.activeTaskCount ?? 0}</dd></div>
		<div><dt class="text-xs text-muted-foreground">{text.completed}</dt><dd class="tabular-nums">{counted?.completeTaskCount ?? 0}</dd></div>
		<div><dt class="text-xs text-muted-foreground">{text.distance}</dt><dd class="font-medium tabular-nums">{counted?.distance ?? counted?.score ?? 0}</dd></div>
	</dl>
{/snippet}

<Card.Root>
	<Card.Header>
		<Card.Title>{text.title}</Card.Title>
	</Card.Header>
	<Card.Content>
		<MemberTable
			{members}
			{text}
			minimumWidth="min-w-[760px]"
			extraHeaders={workHeaders}
			extraCells={workCells}
			mobileExtra={mobileWork}
			extraColumnCount={3}
		/>
	</Card.Content>
</Card.Root>
