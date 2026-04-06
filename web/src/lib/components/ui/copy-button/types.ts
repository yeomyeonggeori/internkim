import type { Snippet } from 'svelte';
import type { ButtonSize, ButtonVariant } from '$lib/components/ui/button/button.svelte';
import type { UseClipboard } from '$lib/hooks/use-clipboard.svelte';
import type { HTMLAttributes } from 'svelte/elements';
import type { WithChildren, WithoutChildren } from 'bits-ui';

export type CopyButtonPropsWithoutHTML = WithChildren<{
	ref?: HTMLButtonElement | null;
	text: string;
	icon?: Snippet<[]>;
	animationDuration?: number;
	onCopy?: (status: UseClipboard['status']) => void;
	size?: ButtonSize;
	variant?: ButtonVariant;
}>;

export type CopyButtonProps = CopyButtonPropsWithoutHTML &
	WithoutChildren<HTMLAttributes<HTMLButtonElement>>;
