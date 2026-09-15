-- 20260914000005 kept the answer of every keyed record write so a retry of the
-- same tool call could read it back. No caller re-sends a tool call under the
-- same observation, so nothing ever read it, and #1706 took out the code that
-- wrote it.

drop table public.idempotency_key;
