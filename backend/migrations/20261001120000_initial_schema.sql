-- Initial schema: accounts, profiles, items, trade offers, moderation, push.
-- There is no price, currency or payment anywhere in this model, by design.

create type item_condition as enum ('new', 'good', 'used');
create type item_status    as enum ('draft', 'active', 'reserved', 'traded', 'removed', 'deleted');
create type offer_status   as enum ('pending', 'accepted', 'declined', 'cancelled', 'expired', 'completed');
create type offer_side     as enum ('offered', 'requested');
create type report_reason  as enum ('spam', 'prohibited_item', 'offensive', 'scam', 'other');
create type report_status  as enum ('open', 'resolved', 'dismissed');

create function set_updated_at() returns trigger
language plpgsql as $$
begin
    new.updated_at := now();
    return new;
end;
$$;

-- Login identities. Either a password (email fallback) or an Apple subject.
create table users (
    id            uuid primary key default gen_random_uuid(),
    email         text,
    password_hash text,
    apple_sub     text unique,
    created_at    timestamptz not null default now(),
    deleted_at    timestamptz,
    constraint users_has_login check (password_hash is not null or apple_sub is not null or deleted_at is not null)
);
create unique index users_email_key on users (lower(email)) where email is not null;

create table profiles (
    id                uuid primary key references users (id),
    display_name      text not null unique,
    avatar_path       text,
    postal_code       text not null,
    -- Centroid of the postal code, never a street address (invariant 8).
    geo_lat           double precision,
    geo_lon           double precision,
    terms_accepted_at timestamptz not null,
    created_at        timestamptz not null default now(),
    deleted_at        timestamptz,
    constraint profiles_geo_pair check ((geo_lat is null) = (geo_lon is null))
);

create table items (
    id          uuid primary key default gen_random_uuid(),
    owner_id    uuid not null references profiles (id),
    title       text not null check (length(trim(title)) > 0),
    description text,
    category    text not null,
    condition   item_condition not null,
    wanted_text text,
    status      item_status not null default 'draft',
    -- Set exactly while status = 'reserved'. One column per item means an
    -- item can structurally belong to at most one accepted offer (invariant 1).
    reserved_by_offer_id uuid,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    constraint items_reservation_matches_status
        check ((status = 'reserved') = (reserved_by_offer_id is not null))
);
create index items_owner_idx on items (owner_id);
create index items_active_idx on items (created_at desc) where status = 'active';

create trigger items_set_updated_at before update on items
    for each row execute function set_updated_at();

-- Guards the item state machine and invariant 5 regardless of which code path
-- writes the row.
create function guard_item_update() returns trigger
language plpgsql as $$
begin
    if new.status is distinct from old.status and not (
           (old.status = 'draft'    and new.status in ('active', 'deleted'))
        or (old.status = 'active'   and new.status in ('reserved', 'deleted', 'removed'))
        or (old.status = 'reserved' and new.status in ('active', 'traded'))
    ) then
        raise exception 'invalid item status transition % -> %', old.status, new.status
            using errcode = 'check_violation';
    end if;

    if old.status not in ('draft', 'active') and (
           new.title       is distinct from old.title
        or new.description is distinct from old.description
        or new.category    is distinct from old.category
        or new.condition   is distinct from old.condition
        or new.wanted_text is distinct from old.wanted_text
        or new.owner_id    is distinct from old.owner_id
    ) then
        raise exception 'item % is % and cannot be edited', old.id, old.status
            using errcode = 'check_violation';
    end if;

    return new;
end;
$$;

create trigger items_guard_update before update on items
    for each row execute function guard_item_update();

create table item_images (
    id           uuid primary key default gen_random_uuid(),
    item_id      uuid not null references items (id) on delete cascade,
    storage_path text not null,
    position     smallint not null check (position >= 0),
    unique (item_id, position)
);

create table trade_offers (
    id                     uuid primary key default gen_random_uuid(),
    initiator_id           uuid not null references profiles (id),
    responder_id           uuid not null references profiles (id),
    target_item_id         uuid not null references items (id),
    message                text,
    status                 offer_status not null default 'pending',
    expires_at             timestamptz not null default now() + interval '14 days',
    accepted_at            timestamptz,
    initiator_confirmed_at timestamptz,
    responder_confirmed_at timestamptz,
    created_at             timestamptz not null default now(),
    constraint trade_offers_distinct_parties check (initiator_id <> responder_id)
);
create index trade_offers_initiator_idx on trade_offers (initiator_id, created_at desc);
create index trade_offers_responder_idx on trade_offers (responder_id, created_at desc);
create index trade_offers_pending_expiry_idx on trade_offers (expires_at) where status = 'pending';

alter table items
    add constraint items_reserved_by_offer_fk
    foreign key (reserved_by_offer_id) references trade_offers (id);

create table trade_offer_items (
    offer_id uuid not null references trade_offers (id) on delete cascade,
    item_id  uuid not null references items (id),
    side     offer_side not null,
    primary key (offer_id, item_id)
);
create index trade_offer_items_item_idx on trade_offer_items (item_id);

create table reports (
    id           uuid primary key default gen_random_uuid(),
    reporter_id  uuid not null references profiles (id),
    subject_type text not null check (subject_type in ('item', 'profile')),
    subject_id   uuid not null,
    reason       report_reason not null,
    note         text,
    status       report_status not null default 'open',
    created_at   timestamptz not null default now(),
    resolved_at  timestamptz,
    resolved_by  uuid references profiles (id)
);
create index reports_open_idx on reports (created_at) where status = 'open';

create table blocks (
    blocker_id uuid not null references profiles (id),
    blocked_id uuid not null references profiles (id),
    created_at timestamptz not null default now(),
    primary key (blocker_id, blocked_id),
    constraint blocks_not_self check (blocker_id <> blocked_id)
);
create index blocks_blocked_idx on blocks (blocked_id);

create table device_tokens (
    id           uuid primary key default gen_random_uuid(),
    user_id      uuid not null references profiles (id),
    apns_token   text not null unique,
    created_at   timestamptz not null default now(),
    last_seen_at timestamptz not null default now()
);

-- Transactional outbox: domain changes and the notifications they cause are
-- committed together; the push sender delivers rows with sent_at is null.
create table notifications (
    id         uuid primary key default gen_random_uuid(),
    user_id    uuid not null references profiles (id),
    kind       text not null,
    offer_id   uuid references trade_offers (id),
    created_at timestamptz not null default now(),
    sent_at    timestamptz
);
create index notifications_unsent_idx on notifications (created_at) where sent_at is null;
