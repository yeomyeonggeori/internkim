<script lang="ts">
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';

	type Crumb = { code: string; name: string };

	let { crumbs, onOpen }: { crumbs: Crumb[]; onOpen: (code: string) => void } = $props();
</script>

<Breadcrumb.Root
	><Breadcrumb.List
		>{#each crumbs as crumb, index (crumb.code)}{#if index > 0}<Breadcrumb.Separator
				/>{/if}<Breadcrumb.Item
				>{#if index === crumbs.length - 1}<Breadcrumb.Page>{crumb.name}</Breadcrumb.Page
					>{:else}<Breadcrumb.Link onclick={() => onOpen(crumb.code)}>{crumb.name}</Breadcrumb.Link
					>{/if}</Breadcrumb.Item
			>{/each}</Breadcrumb.List
	></Breadcrumb.Root
>
