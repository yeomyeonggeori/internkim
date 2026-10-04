Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);

const { personPicture } = await import('$lib/stores/person-picture.svelte');

console.log(JSON.stringify({
	member: personPicture.pictureOf({ memberID: 'member-1' }),
	email: personPicture.pictureOf({ email: 'sample@example.com' }),
	external: personPicture.pictureOfExternal('an-account-nobody-asked-after')
}));

export {};
