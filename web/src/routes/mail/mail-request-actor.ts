export function normalizeMailActorEmail(actorEmail: string) {
	return actorEmail.trim().toLowerCase();
}

export function resolveMailActorEmail(
	accountEmail: string,
	draftEmail: string,
	developmentActorEmail = configuredDevelopmentMailActorEmail()
) {
	return normalizeMailActorEmail(accountEmail || draftEmail || developmentActorEmail);
}

function configuredDevelopmentMailActorEmail() {
	return import.meta.env?.VITE_DEV_USER_EMAIL ?? '';
}
