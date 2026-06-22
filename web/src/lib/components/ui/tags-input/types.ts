import type { HTMLInputAttributes } from 'svelte/elements';

export type TagsInputPropsWithoutHTML = {
	value?: string[];
	validate?: (val: string, tags: string[]) => string | undefined;
	onValueChange?: (value: string[]) => void;
	suggestions?: string[];
	filterSuggestions?: (inputValue: string, suggestions: string[]) => string[];
	restrictToSuggestions?: boolean;
	showSelectedTags?: boolean;
	suggestionsPlacement?: 'top' | 'bottom';
};

export type TagsInputProps = TagsInputPropsWithoutHTML & Omit<HTMLInputAttributes, 'value'>;
