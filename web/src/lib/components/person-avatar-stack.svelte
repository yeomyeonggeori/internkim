<script lang="ts">
	import * as Avatar from '$lib/components/ui/avatar/index.js';
	import type { Snippet } from 'svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';

	type StackPerson = { name: string; seed?: string; email?: string; image?: string; memberID?: string; externalID?: string };

	let {
		people,
		max = 3,
		class: className,
		avatarClass = 'size-7 border-2 border-background',
		label,
		renderPerson
	}: {
		people: StackPerson[];
		max?: number;
		class?: string;
		avatarClass?: string;
		label?: string;
		renderPerson?: Snippet<[StackPerson, number]>;
	} = $props();

	const visiblePeople = $derived(people.slice(0, max));
	const remainingCount = $derived(Math.max(people.length - visiblePeople.length, 0));
</script>

<Avatar.Group class={className} aria-label={label}>
	{#each visiblePeople as person, index (person.memberID ?? person.email ?? person.seed ?? person.name ?? index)}
		{#if renderPerson}
			{@render renderPerson(person, index)}
		{:else}
		<PersonAvatar
			name={displayPersonName(person.name)}
			email={person.email ?? ''}
			seed={person.seed ?? person.name}
			image={person.image ?? ''}
			memberID={person.memberID ?? ''}
			externalID={person.externalID ?? ''}
			class={avatarClass}
		/>
		{/if}
	{/each}
	{#if remainingCount > 0}
		<Avatar.GroupCount data-testid="avatar-stack-overflow" class={cn('text-[9px] font-semibold', avatarClass)}>+{remainingCount}</Avatar.GroupCount>
	{/if}
</Avatar.Group>
