const mattermostBaseURL = process.env.MATTERMOST_BASE_URL || "http://127.0.0.1:8065";
const username = process.env.MATTERMOST_USERNAME || "test";
const password = process.env.MATTERMOST_PASSWORD || "test";
const botToken = process.env.MATTERMOST_BOT_TOKEN || "";
const adminToken = process.env.MATTERMOST_ADMIN_TOKEN || "";
const timeoutMilliseconds = Number(process.env.MATTERMOST_TYPING_TIMEOUT_MS || "120000");
const marker = `INTERNKIM_TYPING_VERIFY_${Date.now()}`;
const testMessage = "안녕. 짧게 인사로 답해줘.";

function requireSuccessfulResponse(response, operation) {
  if (response.ok) return response;
  throw new Error(`${operation} failed with HTTP ${response.status}`);
}

async function login() {
  const response = await fetch(`${mattermostBaseURL}/api/v4/users/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ login_id: username, password }),
  });
  requireSuccessfulResponse(response, "Mattermost login");
  return { token: response.headers.get("Token"), user: await response.json() };
}

function createAuthorizedFetch(token) {
  return (path, options = {}) => fetch(`${mattermostBaseURL}${path}`, {
    ...options,
    headers: { ...options.headers, Authorization: `Bearer ${token}` },
  });
}

async function resolveDirectChannel(authorizedFetch, userID, botUserID) {
  const response = await authorizedFetch("/api/v4/channels/direct", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify([userID, botUserID]),
  });
  requireSuccessfulResponse(response, "Mattermost direct channel lookup");
  return response.json();
}

async function publishTestMessage(authorizedFetch, channelID) {
  const response = await authorizedFetch("/api/v4/posts", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ channel_id: channelID, message: testMessage }),
  });
  requireSuccessfulResponse(response, "Mattermost test post");
  return response.json();
}

function observeTypingAndReply({ token, authorizedFetch, channelID, botUserID, observation }) {
  return new Promise((resolve, reject) => {
    const webSocketURL = `${mattermostBaseURL.replace(/^http/, "ws")}/api/v4/websocket`;
    const socket = new WebSocket(webSocketURL, { headers: { Authorization: `Bearer ${token}` } });
    const timeout = setTimeout(() => finish(), timeoutMilliseconds);
    let isFinished = false;

    function finish(errorValue) {
      if (isFinished) return;
      isFinished = true;
      clearTimeout(timeout);
      socket.close();
      if (errorValue) reject(errorValue);
      else resolve(observation);
    }

    socket.addEventListener("error", event => finish(new Error(`Mattermost WebSocket failed: ${event.message || "unknown error"}`)));
    socket.addEventListener("message", event => {
      Promise.resolve(handleEvent(event)).catch(finish);
    });

    async function handleEvent(event) {
      const document = JSON.parse(String(event.data));
      if (document.event === "hello" && observation.startedAt === 0) {
        observation.startedAt = Date.now();
        const post = await publishTestMessage(authorizedFetch, channelID);
        observation.requestPostID = post.id;
        return;
      }
      if (document.event === "typing" && document.data?.user_id === botUserID && document.broadcast?.channel_id === channelID && observation.typingAt === 0) {
        observation.typingAt = Date.now();
        return;
      }
      if (document.event !== "posted") return;
      const post = JSON.parse(document.data?.post || "{}");
      if (post.user_id !== botUserID || post.channel_id !== channelID || post.root_id !== observation.requestPostID || post.create_at < observation.startedAt) return;
      observation.replyPostID = post.id;
      observation.replyAt = Date.now();
      finish();
    }
  });
}

async function deletePost(authorizedFetch, postID) {
  if (!postID) return false;
  const response = await authorizedFetch(`/api/v4/posts/${postID}`, { method: "DELETE" });
  return response.ok || response.status === 404;
}

const { token, user } = await login();
if (!token) throw new Error("Mattermost login did not return a session token");
const authorizedFetch = createAuthorizedFetch(token);
const botResponse = await authorizedFetch("/api/v4/users/username/internkim");
requireSuccessfulResponse(botResponse, "InternKim account lookup");
const botUser = await botResponse.json();
const channel = await resolveDirectChannel(authorizedFetch, user.id, botUser.id);
const observation = { requestPostID: "", replyPostID: "", startedAt: 0, typingAt: 0, replyAt: 0 };
let observationError;
try {
  await observeTypingAndReply({ token, authorizedFetch, channelID: channel.id, botUserID: botUser.id, observation });
} catch (errorValue) {
  observationError = errorValue;
}
const requestDeleted = await deletePost(authorizedFetch, observation.requestPostID);
const replyDeletionToken = adminToken || botToken;
const replyAuthorizedFetch = createAuthorizedFetch(replyDeletionToken);
const replyDeleted = replyDeletionToken ? await deletePost(replyAuthorizedFetch, observation.replyPostID) : false;

const result = {
  marker,
  channelID: channel.id,
  requestPostID: observation.requestPostID,
  replyPostID: observation.replyPostID,
  requestDeleted,
  replyDeleted,
  typingObserved: observation.typingAt > 0,
  typingDelayMilliseconds: observation.typingAt ? observation.typingAt - observation.startedAt : null,
  replyObserved: observation.replyAt > 0,
  replyDelayMilliseconds: observation.replyAt ? observation.replyAt - observation.startedAt : null,
};

const failures = [];
if (observationError) failures.push(observationError.message);
if (!result.typingObserved) failures.push("InternKim typing event was not observed");
if (!result.replyObserved) failures.push("InternKim reply event was not observed");
if (!result.requestDeleted) failures.push("Mattermost typing request test post was not deleted");
if (replyDeletionToken && !result.replyDeleted) failures.push("Mattermost typing reply test post was not deleted");
if (failures.length > 0) {
  console.error(JSON.stringify(result));
  throw new Error(failures.join("; "));
}
console.log(JSON.stringify(result));
