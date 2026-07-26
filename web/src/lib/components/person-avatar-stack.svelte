<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { cn } from '$lib/utils';

	type StackPerson = { name: string; seed?: string; email?: string; image?: string };

	let {
		people,
		max = 3,
		class: className,
		avatarClass = 'size-7 border-2 border-background',
		label
	}: {
		people: StackPerson[];
		max?: number;
		class?: string;
		avatarClass?: string;
		label?: string;
	} = $props();

	const visiblePeople = $derived(people.slice(0, max));
	const remainingCount = $derived(Math.max(people.length - visiblePeople.length, 0));
</script>

<div class={cn('flex shrink-0 items-center -space-x-2', className)} aria-label={label}>
	{#each visiblePeople as person, index (person.seed ?? person.name ?? index)}
		<PersonAvatar
			name={person.name}
			email={person.email ?? ''}
			seed={person.seed ?? person.name}
			image={person.image ?? ''}
			class={avatarClass}
		/>
	{/each}
	{#if remainingCount > 0}
		<span
			class={cn(
				'grid place-items-center rounded-full bg-muted text-[10px] font-semibold text-muted-foreground',
				avatarClass
			)}
		>
			+{remainingCount}
		</span>
	{/if}
</div>
