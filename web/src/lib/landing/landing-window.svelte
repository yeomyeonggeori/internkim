<script lang="ts">
	import type { Snippet } from 'svelte';
	import { appSectionIcons, type AppSection } from '$lib/components/app-section-icons';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';

	let { section, class: className, children }: { section: AppSection; class?: string; children: Snippet } = $props();

	const shellText = createPageText(appShellText);
	const SectionIcon = $derived(appSectionIcons[section]);
	const title = $derived(shellText[section]);
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
