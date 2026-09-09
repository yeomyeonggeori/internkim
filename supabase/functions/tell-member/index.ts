import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { notifyMember } from '../_shared/notify-member.ts';
import { callingPlane } from '../_shared/plane-caller.ts';
import { tellingAsked, type Asked } from '../_shared/tell-one-member.ts';
import { pushKeysFromVault, reachesSomeDevice } from '../_shared/push-keys.ts';

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const record = await callingPlane(request);
		const pushKeys = await pushKeysFromVault(record);
		if (!reachesSomeDevice(pushKeys)) refuse(503, 'this deployment holds no push keys, so it cannot send notifications yet');

		const telling = tellingAsked((await askedObject(request)) as Asked, crypto.randomUUID());
		if (!telling) refuse(400, 'a member, a known category and a title are what it takes');

		const now = Math.floor(Date.now() / 1000);
		const delivery = await notifyMember(
			record,
			telling.memberID,
			telling.category,
			{ title: telling.title, body: telling.body, openPath: telling.openPath, tag: telling.tag },
			pushKeys,
			now
		);
		return json({ reached: delivery.reached, pruned: delivery.pruned, silent: delivery.silent });
	})
);
