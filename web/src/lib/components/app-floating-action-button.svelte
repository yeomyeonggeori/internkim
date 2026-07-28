<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { floatingAction } from '$lib/stores/floating-action.svelte';
	import type { Snippet } from 'svelte';

	type Props = {
		label: string;
		disabled?: boolean;
		onclick: () => void;
		children: Snippet;
	};

	let { label, disabled = false, onclick, children }: Props = $props();

	$effect(() => {
		floatingAction.isPresent = true;
		return () => {
			floatingAction.isPresent = false;
		};
	});
</script>

<TooltipIconButton
	{label}
	side="left"
	class="internkim-app-floating-action size-14 rounded-full shadow-lg [&_svg:not([class*='size-'])]:size-5"
	size="icon"
	{disabled}
	{onclick}
>
	{@render children()}
</TooltipIconButton>
