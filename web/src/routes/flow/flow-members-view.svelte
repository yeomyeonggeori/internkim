<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import type { FlowMember } from './flow-types';

	type MembersText = {
		title: string;
		name: string;
		email: string;
		hireDate: string;
		role: string;
	mattermost: string;
	active: string;
	completed: string;
	distance: string;
	empty: string;
};

	type Props = {
		members: FlowMember[];
		text: MembersText;
	};

	let { members, text }: Props = $props();
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.title}</Card.Title>
	</Card.Header>
	<Card.Content>
		<div class="overflow-hidden rounded-lg border">
			<Table.Root class="min-w-[820px]">
				<Table.Header class="bg-muted/40">
					<Table.Row class="hover:bg-transparent">
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.name}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.email}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.hireDate}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.role}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.mattermost}</Table.Head>
						<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.active}</Table.Head>
						<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.completed}</Table.Head>
						<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.distance}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each members as member}
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
							<Table.Cell>{member.mattermostStatus}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{member.activeTaskCount}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{member.completeTaskCount}</Table.Cell>
							<Table.Cell class="text-right font-medium tabular-nums">{member.distance ?? member.score ?? 0}</Table.Cell>
						</Table.Row>
					{/each}
					{#if members.length === 0}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell colspan={8} class="py-10 text-center text-muted-foreground">{text.empty}</Table.Cell>
						</Table.Row>
					{/if}
				</Table.Body>
			</Table.Root>
		</div>
	</Card.Content>
</Card.Root>
