import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import {
	aTurnStartingWork,
	changingNothingTheCheckCanRead,
	everyScriptWasAskedAndNothingElse,
	expectedChangesSchemaName,
	replyingAndFinishing,
	turnRouterSchemaName,
	turnWordsOwingOnlyTheReply,
	type Ask
} from './a-model-nobody-pays-for';

type RenderedRung = { endpoint?: string; model: string; apiKeyPath?: string; dimensions?: number };
type RenderedLanguageModel = {
	tiers?: Record<string, RenderedRung[]>;
	embedding?: RenderedRung;
	decision?: RenderedRung;
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

function renderedMemory(plane: ACompanyPlane): { embeddingModel?: string; embeddingDimensions?: number } {
	const rendered = JSON.parse(readFileSync(plane.runtimeConfigurationPath, 'utf8')) as {
		memory?: { embeddingModel?: string; embeddingDimensions?: number };
	};
	return rendered.memory ?? {};
}

async function until(satisfied: () => Promise<boolean>, seconds: number): Promise<boolean> {
	for (let attempt = 0; attempt < seconds * 2; attempt += 1) {
		if (await satisfied()) return true;
		await Bun.sleep(500);
	}
	return satisfied();
}

async function theFirstAsk(plane: ACompanyPlane, kind: Ask['kind']): Promise<Ask | undefined> {
	const asked = await plane.model.asked();
	return asked.find((ask) => ask.kind === kind);
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

		expect(
			languageModel.embedding?.endpoint,
			'embeddings go through capabilityd, so the rung names no endpoint of its own'
		).toBeUndefined();
		expect(languageModel.embedding?.model).toBe('google/embeddinggemma-2');
		expect(renderedMemory(plane).embeddingModel).toBe('google/embeddinggemma-2');
		expect(renderedMemory(plane).embeddingDimensions).toBe(768);
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
		const request = 'say hello';
		await plane.model.decideTurn(aTurnStartingWork(request, []));
		await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
		await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
		await plane.model.callNext('reply', replyingAndFinishing('안녕하세요'));
		const started = await fetch(`${plane.blueclawURL}/admin/api/run/start`, {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({
				requesterPersonID: plane.people[0].memberID,
				prompt: request
			})
		});
		expect(started.status, await started.clone().text()).toBe(200);

		const wasAsked = await until(async () => (await theFirstAsk(plane, 'completion')) !== undefined, 60);
		expect(wasAsked, 'the endpoint the plane rendered was never asked for a completion').toBe(true);

		const laddersModels = new Set(
			everyTier.flatMap((tier) =>
				(renderedLanguageModel(plane).tiers?.[tier] ?? []).map((rung) => rung.model)
			)
		);
		const firstAsk = await theFirstAsk(plane, 'completion');
		expect(
			laddersModels.has(firstAsk?.model ?? ''),
			`the turn asked for ${firstAsk?.model}, which no rendered tier names`
		).toBe(true);
		expect(
			firstAsk?.authorization,
			'the endpoint was asked without the key its rung named'
		).toStartWith('Bearer ');

		const firstDecision = await theFirstAsk(plane, 'decision');
		expect(
			firstDecision?.model,
			'intake decided the turn without the decision model the plane rendered'
		).toBe(renderedLanguageModel(plane).decision?.model);
		expect(firstDecision?.authorization, 'the decisions endpoint was asked without a key').toStartWith('Bearer ');
		await everyScriptWasAskedAndNothingElse(plane.model);
	} finally {
		await plane.stop();
	}
}, 180_000);
