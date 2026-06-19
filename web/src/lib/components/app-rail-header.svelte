<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
</script>

<Sidebar.Header class="gap-0 p-0">
	<a
		href="/admin/"
		aria-label="Blueclaw"
		class="mx-2.5 mb-1 flex h-10 w-10 items-center gap-3 overflow-hidden rounded-md text-sm font-semibold text-sidebar-accent-foreground transition-[width] duration-150 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring group-hover:w-[204px] group-focus-within:w-[204px] group-data-[profile-open=true]:w-[204px]"
		data-sveltekit-preload-data="off"
		data-sveltekit-preload-code="off"
	>
		<span class="flex size-10 shrink-0 items-center justify-center rounded-md bg-background ring-1 ring-sidebar-border">
			<img src="/logo.svg" alt="" class="size-7" />
		</span>
		<span class="min-w-0 max-w-0 truncate opacity-0 transition-[max-width,opacity] duration-150 ease-out group-hover:max-w-[148px] group-hover:opacity-100 group-focus-within:max-w-[148px] group-focus-within:opacity-100 group-data-[profile-open=true]:max-w-[148px] group-data-[profile-open=true]:opacity-100">Blueclaw</span>
	</a>
</Sidebar.Header>
