update public.agent set revoked_at = now()
where name = 'day digest' and revoked_at is null;

delete from vault.secrets where name = 'day_digest_agent_key';
