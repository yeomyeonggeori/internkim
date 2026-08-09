export type HomeRedirectQuestion = {
	isBuilding: boolean;
	projectURL: string;
	publishableKey: string;
	pathname: string;
};

function servesCompanies(projectURL: string, publishableKey: string): boolean {
	return Boolean(projectURL && publishableKey);
}

export function sendsHomeToFlow(question: HomeRedirectQuestion): boolean {
	if (question.isBuilding) return false;
	if (question.pathname !== '/') return false;
	return servesCompanies(question.projectURL, question.publishableKey);
}
