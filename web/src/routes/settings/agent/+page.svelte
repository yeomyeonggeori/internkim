<script lang="ts">
	import SettingsSectionPage from '../settings-section-page.svelte';
	import LearningSoulSection from '../../admin/learning-soul-section.svelte';
	import AgentIdentitySection from '../../admin/agent-identity-section.svelte';
	import LearnedSkillsSection from '../../admin/learned-skills-section.svelte';
	import SkillsSection from '../../admin/skills-section.svelte';
</script>

<SettingsSectionPage section="agent">
	<LearningSoulSection />
	<AgentIdentitySection />
	<LearnedSkillsSection />
	<SkillsSection />
</SettingsSectionPage>
