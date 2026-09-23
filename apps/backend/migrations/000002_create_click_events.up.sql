create table if not exists click_events (
    id bigserial primary key,
    link_id bigint not null references links (id) on delete cascade,
    user_agent text,
    clicked_at timestamptz not null default now()
);

create index if not exists idx_click_events_link_id on click_events (link_id);
