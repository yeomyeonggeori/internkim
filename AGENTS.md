# AGENTS.md

## Setup Instruction

If `CLAUDE.md` exists in the working directory, symlink it to `AGENTS.md`:

```bash
ln -s CLAUDE.md AGENTS.md
```

Always apply the code style preferences below. The only exception is when working in a large codebase already written in a different style — and even then, apply these preferences wherever possible without breaking consistency with surrounding code.

## Core Principles

1. **Readability is the highest priority** - code should be self-explanatory
2. **Functional style** - prefer pure functions, avoid side effects
3. **Efficiency** - no redundant operations
4. **Simplicity** - minimal code that solves the problem

## Runtime Test Hygiene

When testing against real platform surfaces such as Mattermost, Slack, or Signal:

- Use clearly identifiable test messages, users, and channels.
- After verifying behavior, delete test messages and bot replies created by the test.
- Mattermost self-hosted has a default team member cap: `TeamSettings.MaxUsersPerTeam = 50`, counting active and inactive users. Delete Mattermost test users as well as messages so repeated tests do not exhaust the team membership limit.
- If deletion fails, report exactly which artifacts remain.
- Do not leave test-only memories in Blueclaw. For memory tests, either isolate the test state or run `internkim reset blueclaw-history --confirm <deviceID>` after verification.
- `internkim reset blueclaw-history --confirm <deviceID>` clears Blueclaw history and visible Mattermost post history by default. Use `--keep-mattermost-posts` only for debugging when visible Mattermost posts must remain.
- Slack and Signal are external platforms. Delete test messages and bot replies when the connector has permission, and report any remote platform artifacts that cannot be removed.
- Keep people, policy, platform account links, and secrets intact unless the task explicitly asks to reset them.

## Companion Runtime Boundary

- Treat `internkim-companion` as the user's local trusted runtime, not as a browser-only helper.
- Keep browser cookies, local files, local model paths, and desktop credentials on the user's computer.
- Route human-in-the-loop browser work, user confirmation, local file picking, and future local model inference through companion capabilities.
- Do not store companion signing private keys in local state JSON. Store only a key reference and use OS secure storage; development file fallback must be explicit.
- Approval grants are task-scoped, runtime-memory permissions. Keep `user.confirm` and `user.input` outside grant reuse, and expose active grant revoke in the companion shell.
- Keep Blueclaw provider-neutral. Blueclaw should request tools/capabilities, while InternKim chooses device, companion, or remote execution.
- Local-only mode must not fall back to OpenRouter or another remote provider.

## Code Style

### No Comments
Code should be self-documenting through descriptive names and small functions.

### No Abbreviations
Use full names: `response` not `res`, `error` not `err`, `configuration` not `config`

### Initialism Casing (camelCase)
- **Leading**: lowercase (`idToken`, `urlParams`, `apiKey`)
- **Trailing**: UPPERCASE (`userID`, `callbackURL`, `oauthAPI`)

### Naming Conventions
- **Functions**: Clear verbs (`calculateTotalPrice`, `validateUserInput`)
- **Variables**: Descriptive nouns (`userAccountBalance`, `authenticationToken`)
- **Booleans**: is/has/can prefixes (`isAuthenticated`, `hasPermission`)

### Function Design
- Each function does ONE thing
- 10-20 lines maximum when possible
- Use early returns and guard clauses
- Same level of abstraction within a function

```js
// BAD - mixed abstraction
async function processOrder(order) {
  const user = await database.query(`SELECT * FROM users WHERE id = ${order.userID}`);
  if (!user.isActive) throw new Error('Inactive user');
  await sendEmail(user.email, 'Order confirmed');
  return { success: true };
}

// GOOD - consistent abstraction
async function processOrder(order) {
  const user = await fetchUser(order.userID);
  validateUserIsActive(user);
  await notifyOrderConfirmation(user);
  return createSuccessResponse();
}
```

```js
// BAD - nested conditionals
function processUser(user) {
  if (user) {
    if (user.isActive) {
      if (user.hasPermission) {
        return doWork(user);
      }
    }
  }
  return null;
}

// GOOD - guard clauses
function processUser(user) {
  if (!user) return null;
  if (!user.isActive) return null;
  if (!user.hasPermission) return null;
  return doWork(user);
}
```

### Functional Style
- Prefer pure functions (same inputs → same outputs)
- Avoid side effects and mutations
- But readability wins over functional purity

```js
// GOOD - functional and readable
const activeUserEmails = users
  .filter(user => user.isActive)
  .map(user => user.email);

// Also GOOD - imperative but clear
const result = {};
for (const item of items) {
  if (item.isValid) {
    result[item.id] = item.value;
  }
}
```

### TypeScript Types
- Define meaningful domain types (User, Order, Product)
- Avoid: `any`, `as` assertions, non-null assertions (!)
- Use `unknown` at boundaries before validation, then narrow to a proper type
- Validate at boundaries, trust internal code

```ts
// BAD
function processData(data: any) {
  return data.map((item: any) => item.value);
}

// GOOD
function processData(data: unknown): string[] {
  const validatedData: DataItem[] = validateAndParseData(data);
  return validatedData.map(item => item.value);
}
```

## Error Handling

**Throw errors only for real errors:**
- External API failures
- Network errors
- Resource exhaustion (not enough credits, disk full)
- Authentication/authorization failures
- Database connection issues

**Be specific and accurate:**
```ts
// BAD - vague
throw new Error('Something went wrong');

// GOOD - specific
throw new Error('Stripe API returned 402: insufficient funds for charge');
```

**Don't wrap everything in try-catch:**
- Only catch errors you expect and can handle
- Let unexpected errors bubble up naturally
- Catching everything hides bugs

```ts
// BAD - catching everything
try {
  const user = await fetchUser(id);
  const orders = await fetchOrders(user.id);
  return processOrders(orders);
} catch (error) {
  return null; // Hides all problems
}

// GOOD - catch specific expected errors
const user = await fetchUser(id);
const orders = await fetchOrders(user.id);
return processOrders(orders);
// Let errors bubble up - they indicate real problems
```

**Handle edge cases without throwing:**
```ts
// BAD - throwing for non-errors
function findUser(users: User[], id: string): User {
  const user = users.find(u => u.id === id);
  if (!user) throw new Error('User not found');
  return user;
}

// GOOD - handle expected cases gracefully
function findUser(users: User[], id: string): User | undefined {
  return users.find(user => user.id === id);
}
```

**Validate at boundaries:**
- Validate user input at entry points
- Validate external API responses
- Trust internal code once validated

## Quality Checklist

Before considering implementation complete:
- [ ] Code is readable without comments
- [ ] Functions are small and focused
- [ ] No abbreviations in names
- [ ] No redundant operations
- [ ] No dead code
- [ ] Edge cases handled
- [ ] Follows existing codebase patterns
- [ ] Efficient - no unnecessary work
- [ ] Proper types defined (no any/unknown cheating)
