import { Database } from "bun:sqlite";

const durationMilliseconds = Number(process.argv[2] ?? 90000);
const deadline = Date.now() + durationMilliseconds;
const counters = { jitIterations: 0, allocatedMebibytes: 0, sqliteRows: 0, httpRoundTrips: 0, socketRoundTrips: 0, workerReplies: 0 };

function hotLoop(seed: number): number {
  let accumulator = seed;
  for (let index = 0; index < 2_000_000; index++) accumulator = (accumulator * 1103515245 + 12345) & 0x7fffffff;
  return accumulator;
}

function churnHeap(): void {
  const retained: unknown[] = [];
  for (let index = 0; index < 20000; index++) retained.push({ index, text: "x".repeat(index % 300), list: [index, index + 1] });
  const buffer = new ArrayBuffer(64 * 1024 * 1024 + 12345);
  new Uint8Array(buffer).fill(7, 0, buffer.byteLength);
  counters.allocatedMebibytes += 64;
  Bun.gc(false);
}

const database = new Database(":memory:");
database.run("create table sample (id integer primary key, body text)");
const insert = database.prepare("insert into sample (body) values (?)");

const server = Bun.serve({
  port: 0,
  fetch(request, serverHandle) {
    if (serverHandle.upgrade(request)) return;
    return new Response(new Uint8Array(200_000).fill(1));
  },
  websocket: { message(socket, message) { socket.send(message); } },
});

const worker = new Worker(URL.createObjectURL(new Blob([
  "self.onmessage = (event) => { let total = 0; for (let i = 0; i < 5e6; i++) total += i % event.data; self.postMessage(total); };",
])));

async function socketRoundTrip(): Promise<void> {
  const socket = new WebSocket(`ws://127.0.0.1:${server.port}`);
  await new Promise((resolve) => (socket.onopen = resolve));
  const echoed = new Promise((resolve) => (socket.onmessage = resolve));
  socket.send("x".repeat(100_000));
  await echoed;
  socket.close();
  counters.socketRoundTrips++;
}

async function workerRoundTrip(): Promise<void> {
  const reply = new Promise((resolve) => (worker.onmessage = resolve));
  worker.postMessage(7);
  await reply;
  counters.workerReplies++;
}

while (Date.now() < deadline) {
  hotLoop(counters.jitIterations);
  counters.jitIterations++;
  churnHeap();
  for (let index = 0; index < 2000; index++) insert.run("row".repeat(20));
  counters.sqliteRows += 2000;
  const response = await fetch(`http://127.0.0.1:${server.port}/`);
  if ((await response.arrayBuffer()).byteLength !== 200_000) throw new Error("short HTTP body");
  counters.httpRoundTrips++;
  await socketRoundTrip();
  await workerRoundTrip();
}

worker.terminate();
server.stop(true);
console.log(JSON.stringify({ bun: Bun.version, rssMebibytes: Math.round(process.memoryUsage().rss / 1048576), ...counters }));
console.log("PAGE-STRESS-DONE");
