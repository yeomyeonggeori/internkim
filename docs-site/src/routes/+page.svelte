<script lang="ts">
	import { docs } from 'svelte-docsmith/content';
	import { DocsShell, Hero, FeatureGrid, Feature, CTA, Action } from 'svelte-docsmith';
	import { siteConfig } from '$lib/site-config';
</script>

<DocsShell config={siteConfig} content={docs} layout="page" pattern>
	<Hero
		title="InternKim"
		description="A coworker your company runs itself. It works from the messenger you already use, on a computer you already leave on."
	>
		{#snippet actions()}
			<Action href="/docs/introduction">Read the docs</Action>
			<Action href="/docs/architecture" variant="secondary">See the architecture</Action>
		{/snippet}
	</Hero>

	<FeatureGrid
		background="muted"
		title="How it is put together"
		description="Two planes, and one rule deciding which is which."
	>
		<Feature title="Two clients, one conversation">
			The web app is the default and holds everything, chat screen included. Your
			messenger's own apps reach the same conversation from a phone.
		</Feature>
		<Feature title="Your messenger stays yours">
			InternKim attaches to the Slack, Mattermost, or Buzz relay you already run. No
			conversation is stored centrally, because there is nowhere central to put one.
		</Feature>
		<Feature title="Nothing listens on your computer">
			Every connection the host makes points outward. No tunnel, no forwarded port, no
			hostname aimed at a machine in your office.
		</Feature>
		<Feature title="No special hardware">
			A Linux server, a Mac that does not sleep, a box in a cupboard. Whatever stays on.
		</Feature>
		<Feature title="Self-hostable end to end">
			The schema is a directory of migrations and the app is a static build. Point the host
			at a hosted project or at one you run yourself.
		</Feature>
		<Feature title="One boundary per question">
			The database decides who may read a record. The operating system decides who may
			open a file. Nothing else adjudicates access.
		</Feature>
		<Feature title="The agent acts as a person">
			It gets a session for whoever it is working for and sees exactly what they see.
		</Feature>
		<Feature title="Your model, your choice">
			The agent talks to whichever provider you configure, or to a model on the box.
		</Feature>
	</FeatureGrid>

	<CTA
		title="Start with the shape"
		description="The architecture page has the whole system in one diagram."
	>
		{#snippet actions()}
			<Action href="/docs/architecture">Architecture</Action>
		{/snippet}
	</CTA>
</DocsShell>
