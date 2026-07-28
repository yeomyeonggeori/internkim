export const breadcrumbMeta = $state<{ value: string; clear: (() => void) | undefined }>({
	value: '',
	clear: undefined
});
