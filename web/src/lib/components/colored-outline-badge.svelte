<script lang="ts">
 import type { ComponentProps } from 'svelte';
 import { Badge } from '$lib/components/ui/badge';
 import { cn } from '$lib/utils';
 let { color = '', class: className, style, children, ...restProps }: Omit<ComponentProps<typeof Badge>, 'variant'> & { color?: string } = $props();
</script>
<Badge
 {...restProps}
 variant="outline"
 class={cn(color && 'border-[color-mix(in_oklch,var(--badge-color)_45%,hsl(var(--border)))] text-[color-mix(in_oklch,var(--badge-color)_45%,hsl(var(--foreground)))]', className)}
 style={color ? `--badge-color: ${color}; ${style ?? ''}` : style}
>
 {@render children?.()}
</Badge>
