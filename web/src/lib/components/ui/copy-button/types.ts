import type { Snippet } from 'svelte';
import type { ButtonSize, ButtonVariant } from '$lib/components/ui/button/button.svelte';
import type { UseClipboard } from '$lib/hooks/use-clipboard.svelte';
 
export type CopyButtonPropsWithoutHTML = {
	ref?: HTMLButtonElement | null;
	text: string;
	icon?: Snippet<[]>;
	animationDuration?: number;
	onCopy?: (status: UseClipboard['status']) => void;
	size?: ButtonSize;
	variant?: ButtonVariant;
	children?: Snippet;
	class?: string;
	tabindex?: number;
	disabled?: boolean;
};

export type CopyButtonProps = CopyButtonPropsWithoutHTML;
