<script lang="ts">
	import { untrack, type Snippet } from 'svelte';
	import FrameIcon from '@lucide/svelte/icons/frame';
	import { cn } from '$lib/utils';
	import { usePlacement, useWorkspace, type ThemeName, type VariantAxis } from './workspace.svelte';

	let {
		id,
		name,
		source = '',
		note = '',
		variants = [],
		class: className = 'w-80',
		isContained = false,
		children
	}: {
		id: string;
		name: string;
		source?: string;
		note?: string;
		variants?: VariantAxis[];
		class?: string;
		isContained?: boolean;
		children: Snippet<[ThemeName]>;
	} = $props();

	const workspace = useWorkspace();
	const placement = usePlacement();
	const isSelected = $derived(workspace.selectedID === id);

	$effect(() => {
		const frame = { id, name, ...placement(), source, note, variants };
		untrack(() => workspace.register(frame));
		return () => untrack(() => workspace.unregister(frame.id));
	});
</script>

<figure id={id} data-frame={id} data-frame-source={source || undefined} class="m-0 grid max-w-full shrink-0 scroll-m-16 gap-1.5">
	<figcaption>
		<button
			type="button"
			class={cn(
				'text-muted-foreground hover:text-foreground flex max-w-full items-center gap-1 text-[11px] font-medium',
				isSelected && 'text-brand hover:text-brand'
			)}
			onclick={() => workspace.select(id)}
		>
			<FrameIcon class="size-3 shrink-0" />
			<span class="truncate">{name}</span>
		</button>
	</figcaption>
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class={cn(
			'flex flex-wrap outline-offset-2',
			isSelected ? 'outline-brand outline-2' : 'hover:outline-brand/50 hover:outline-1'
		)}
		onpointerdowncapture={() => workspace.select(id)}
	>
		{#each workspace.themes as theme (theme)}
			<div
				data-theme-pane={theme}
				class={cn(
					theme,
					'bg-background text-foreground @container/conversation relative flex max-w-[calc(100vw-3rem)] min-w-0 flex-col gap-3 p-5 shadow-xs',
					isContained && 'transform-[translateZ(0)] overflow-hidden',
					className
				)}
			>
				{@render children(theme)}
			</div>
		{/each}
	</div>
</figure>
