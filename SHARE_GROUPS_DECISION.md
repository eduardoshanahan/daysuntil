# Share Groups Decision

Date: 2026-05-17

## Decision

The application will move away from account-wide public sharing and adopt share groups.

The new public-sharing model will be:

- a user can create one or more share groups
- each share group has:
  - a user-owned name
  - a human-readable public slug
  - zero or more intervals
- public pages will resolve by share-group slug, not by account-wide `public_slug`
- an interval belongs to zero or one share group in the first version
- an interval with no share group is private by default

This replaces the current model where a single account-wide public slug exposes every interval marked `public`.

## Why

The current public model is simple, but it has an important privacy tradeoff:

- one public link exposes all intervals currently marked `public`

That is broader than what many users actually want. In practice, people often want one of these:

1. share one interval
2. share a small related set of intervals
3. keep everything else private

Share groups fit that use case better than either:

- pure account-wide sharing, which is too broad
- pure per-interval links, which are narrow but awkward when several intervals belong together

## Chosen model

### User experience

Examples:

- a group named `Trips`
- a group named `Wedding`
- a group named `House move`

Each group gets one public route such as:

```text
/g/forest-harbor-otter
```

That route shows only the intervals assigned to that group.

This means:

- a user can share one interval by putting just that interval in a group
- a user can share several related intervals together
- a user can revoke exposure by removing intervals from the group, deleting the group, or rotating the group slug later

### Interval membership rule

For the first implementation:

- each interval belongs to at most one share group

This keeps the UI and schema simple:

- no many-to-many join table yet
- no ambiguity about where an interval is shared
- easier editing flow in the current app

If the product later needs the same interval to appear in multiple public collections, the model can be expanded to many-to-many.

## Data model

### New table

Add a `share_groups` table:

```sql
CREATE TABLE share_groups (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  public_slug TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

Indexes:

- unique index on `public_slug`
- index on `user_id`

### Interval changes

Replace account-wide public visibility with group membership.

First-version approach:

- add nullable `share_group_id` to `intervals`
- `NULL` means private
- non-`NULL` means shared through that group

That gives a simple rule:

- private interval: `share_group_id IS NULL`
- shared interval: `share_group_id = some group id`

The current `visibility` field can then be:

1. removed in a migration, or
2. kept temporarily during transition and derived from `share_group_id`

For a safe migration path, keeping it temporarily is simpler.

## Routes

### Public routes

Replace:

- `/p/{public_slug}`
- `/api/public/profiles/{publicSlug}`

With:

- `/g/{groupSlug}`
- `/api/public/groups/{groupSlug}`

The public response should contain:

- share-group name
- owner display name or username if desired
- intervals in that group

### Authenticated routes

Likely new routes:

- `GET /api/share-groups`
- `POST /api/share-groups`
- `PUT /api/share-groups/{id}`
- `DELETE /api/share-groups/{id}`
- `POST /api/share-groups/{id}/rotate`

Interval create/update routes should also accept:

- `share_group_id`

## UI changes

### Private app

The current interval modal uses `private/public`.

That will likely become:

- `Private`
- `Share in group`

If `Share in group` is selected, the user chooses one existing group or creates one.

The profile panel will no longer be the main place for public sharing. Instead:

- group management becomes its own section
- each group shows:
  - name
  - public link
  - rotate link action
  - delete group action

### Public app

The current public page shows a user profile.

That should become a group page:

- group name as the main header
- optional owner label
- only the intervals assigned to that group

## Migration plan

Because the app currently has a single-user real deployment, migration complexity is low.

Suggested transition:

1. add `share_groups`
2. add `share_group_id` to `intervals`
3. keep existing `public_slug` and `visibility` temporarily
4. add group-based routes and UI
5. stop creating new account-wide public links
6. remove the old account-wide public profile UI once group sharing works
7. optionally add a one-time tool to convert current public intervals into a default share group

Since the current deployment is small, an even simpler path is acceptable:

- add the new schema
- stop using the old account-wide sharing model
- manually recreate any wanted public links through groups

## Security impact

This improves privacy because:

- public exposure becomes narrower by default
- sharing several intervals becomes an explicit grouping action
- sharing one interval no longer requires account-wide public state

This does not make shared content secret.

Anyone who knows a group slug can still access that public group. The privacy gain comes from reducing scope, not from turning public links into high-entropy secret tokens.

## Out of scope for version one

Do not add these in the first implementation:

- many-to-many interval/group membership
- public editing or collaboration
- expiring share links
- password-protected public links
- mixed account-wide and group-wide sharing modes

Those can be added later if needed, but they would complicate both the UI and the data model.

## Recommended implementation order

1. add schema and backend model support for share groups
2. add authenticated CRUD for groups
3. add group-based public routes
4. update interval create/edit UI to assign a group instead of `public/private`
5. remove or deprecate account-wide public sharing UI
6. update tests and docs
