alter table public.messenger_person rename to contact;
alter policy messenger_person_readable_by_colleague on public.contact rename to contact_readable_by_colleague;
