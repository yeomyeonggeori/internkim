routerAdd("POST", "/api/site-auth/passkey/register-options", (event) => {
	return require(`${__hooks}/passkey-lib.js`).registerOptions(event);
});
routerAdd("POST", "/api/site-auth/passkey/register", (event) => {
	return require(`${__hooks}/passkey-lib.js`).register(event);
});
routerAdd("POST", "/api/site-auth/passkey/login-options", (event) => {
	return require(`${__hooks}/passkey-lib.js`).loginOptions(event);
});
routerAdd("POST", "/api/site-auth/passkey/login", (event) => {
	return require(`${__hooks}/passkey-lib.js`).login(event);
});
