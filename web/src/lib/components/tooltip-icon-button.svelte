<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import type { ComponentProps, Snippet } from 'svelte';

	type Props = ComponentProps<typeof Button> & {
		label: string;
		side?: ComponentProps<typeof Tooltip.Content>['side'];
		children: Snippet;
	};

	let { label, side = 'bottom', children, ref = $bindable(null), ...buttonProps }: Props = $props();
</script>

<Tooltip.Root>
	<Tooltip.Trigger>
		{#snippet child({ props })}
			<Button bind:ref {...props} {...buttonProps} aria-label={label}>
				{@render children()}
			</Button>
		{/snippet}
	</Tooltip.Trigger>
	<Tooltip.Content {side}>{label}</Tooltip.Content>
</Tooltip.Root>
