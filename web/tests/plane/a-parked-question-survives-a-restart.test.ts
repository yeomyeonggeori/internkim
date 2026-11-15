import { afterAll, afterEach, beforeAll, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import { everyScriptWasAskedAndNothingElse } from './a-model-nobody-pays-for';
import { aQuestionParkedAcrossAnInterruptionIsAskedAnsweredAndCarriedOutOnce } from './a-parked-question';

let plane: ACompanyPlane;

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz', inbound: 'acp' });
}, 180_000);

afterEach(async () => {
	await everyScriptWasAskedAndNothingElse(plane.model);
}, 90_000);

afterAll(async () => {
	await plane?.stop();
});

test('a parked question outlives a blueclaw restart: asked once, answered once, carried out once, one reply', async () => {
	await aQuestionParkedAcrossAnInterruptionIsAskedAnsweredAndCarriedOutOnce(plane, {
		conversationID: `conversation-restart-${plane.runIdentifier}`,
		marker: `평면 재시작 점검 ${Date.now()}`,
		interruption: () => plane.restartBlueclaw()
	});
}, 180_000);
