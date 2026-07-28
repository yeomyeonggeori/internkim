<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { floatingAction } from '$lib/stores/floating-action.svelte';
	import type { ComponentProps, Snippet } from 'svelte';

	type Props = ComponentProps<typeof TooltipIconButton> & {
		label: string;
		children: Snippet;
	};

	let { label, children, ref = $bindable(null), ...buttonProps }: Props = $props();

	$effect(() => {
		floatingAction.isPresent = true;
		return () => {
			floatingAction.isPresent = false;
		};
	});
</script>

<TooltipIconButton
	bind:ref
	{label}
	side="left"
	class="internkim-app-floating-action size-14 rounded-full shadow-lg [&_svg:not([class*='size-'])]:size-5"
	size="icon"
	{...buttonProps}
>
	{@render children()}
</TooltipIconButton>
