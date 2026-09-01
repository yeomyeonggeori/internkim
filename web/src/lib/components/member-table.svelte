<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import type { Snippet } from 'svelte';

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
		text: MemberTableText;
		minimumWidth?: string;
		extraHeaders?: Snippet;
		extraCells?: Snippet<[TabledMember]>;
		extraColumnCount?: number;
	};

	let {
		members,
		text,
		minimumWidth = 'min-w-[560px]',
		extraHeaders,
		extraCells,
		extraColumnCount = 0
	}: Props = $props();

	const headClass = 'h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground';
</script>

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
							<PersonAvatar name={member.name} email={member.email} image={member.image ?? ''} class="size-7" />
							{member.name}
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
					<Table.Cell colspan={4 + extraColumnCount} class="py-10 text-center text-muted-foreground">
						{text.empty}
					</Table.Cell>
				</Table.Row>
			{/if}
		</Table.Body>
	</Table.Root>
</div>
