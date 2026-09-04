import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';

type RenderedRung = { endpoint: string; model: string; apiKeyPath?: string };
type RenderedLanguageModel = {
	tiers?: Record<string, RenderedRung[]>;
	embedding?: RenderedRung;
	capability?: unknown;
};

const everyTier = ['xlow', 'low', 'medium', 'high', 'xhigh', 'max'];

function renderedLanguageModel(plane: ACompanyPlane): RenderedLanguageModel {
	const rendered = JSON.parse(readFileSync(plane.runtimeConfigurationPath, 'utf8')) as {
		languageModel?: RenderedLanguageModel;
	};
	expect(rendered.languageModel, 'the renderer wrote no languageModel block').toBeDefined();
	return rendered.languageModel as RenderedLanguageModel;
}

async function until(satisfied: () => boolean, seconds: number): Promise<boolean> {
	for (let attempt = 0; attempt < seconds * 2; attempt += 1) {
		if (satisfied()) return true;
		await Bun.sleep(500);
	}
	return satisfied();
}

// The plane is where a company's model configuration becomes the agent's. Until
// this change the ladder was compiled into blueclaw and the plane had no say, so
// what this asks is that the document blueclaw boots on names an endpoint per
// tier, and that the endpoint it names is the one this run stood up.
test('every tier is rendered as an endpoint this plane chose', async () => {
	const plane = await aCompanyPlane();
	try {
		const languageModel = renderedLanguageModel(plane);
		expect(
			languageModel.capability,
			'a plane reaches its own endpoints; the capability route is the device path'
		).toBeUndefined();

		for (const tier of everyTier) {
			const rungs = languageModel.tiers?.[tier];
			expect(rungs?.length, `the ${tier} tier was given no endpoint`).toBeGreaterThan(0);
			for (const rung of rungs ?? []) {
				expect(rung.endpoint, `the ${tier} tier reaches somewhere this run did not choose`).toBe(
					plane.model.url
				);
				expect(rung.model, `the ${tier} tier names no model`).toBeTruthy();
				expect(
					rung.apiKeyPath,
					`the ${tier} tier names no key file, so the endpoint would be asked without one`
				).toBeTruthy();
			}
		}

		expect(languageModel.embedding?.endpoint).toBe(plane.model.url);
		expect(languageModel.embedding?.model).toBeTruthy();
	} finally {
		await plane.stop();
	}
}, 180_000);

// A document naming an endpoint is not the same as the agent reaching it. This
// asks blueclaw for one turn and reads the endpoint's own record of being asked,
// which is the only evidence that the loop went through the model client rather
// than through the route this stage took away.
test('the agent asks that endpoint for a model its ladder names', async () => {
	const plane = await aCompanyPlane();
	try {
		const started = await fetch(`${plane.blueclawURL}/admin/api/run/start`, {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({
				requesterPersonID: plane.people[0].memberID,
				prompt: 'say hello'
			})
		});
		expect(started.status, await started.clone().text()).toBe(200);

		const wasAsked = await until(() => plane.model.completions.length > 0, 60);
		expect(wasAsked, 'the endpoint the plane rendered was never asked anything').toBe(true);

		const laddersModels = new Set(
			everyTier.flatMap((tier) =>
				(renderedLanguageModel(plane).tiers?.[tier] ?? []).map((rung) => rung.model)
			)
		);
		const firstAsk = plane.model.completions[0];
		expect(
			laddersModels.has(firstAsk.model),
			`the turn asked for ${firstAsk.model}, which no rendered tier names`
		).toBe(true);
		expect(
			firstAsk.authorization,
			'the endpoint was asked without the key its rung named'
		).toStartWith('Bearer ');
	} finally {
		await plane.stop();
	}
}, 180_000);
