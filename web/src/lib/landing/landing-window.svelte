<script lang="ts">
	import type { Snippet } from 'svelte';
	import { appSectionIcons, type AppSection } from '$lib/components/app-section-icons';
	import { cn } from '$lib/utils';

	let { section, title, class: className, children }: { section: AppSection; title: string; class?: string; children: Snippet } = $props();

	const SectionIcon = $derived(appSectionIcons[section]);
</script>

<figure aria-label={title} class={cn('bg-background overflow-hidden rounded-xl border shadow-sm', className)}>
	<header class="text-muted-foreground flex h-11 items-center gap-2 border-b px-4 text-sm">
		<SectionIcon class="size-4" />
		<span class="text-foreground font-medium">{title}</span>
	</header>
	<div inert class="max-sm:max-h-[34rem] max-sm:overflow-hidden max-sm:[mask-image:linear-gradient(to_bottom,black_85%,transparent)]">
		{@render children()}
	</div>
</figure>
