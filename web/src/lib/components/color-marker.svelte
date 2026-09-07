<script lang="ts">
	import { cn } from '$lib/utils';

	type Props = {
		color?: string;
		class?: string;
		'data-slot'?: string;
	};

	let { color = '', class: className, ...restProps }: Props = $props();
</script>

<span
	class={cn('h-3 w-0.75 shrink-0 rounded-full bg-muted-foreground', className)}
	style:background-color={color || undefined}
	aria-hidden="true"
	{...restProps}
></span>
