# Invited user forced to set a password — report

## The gap

A user created through Supabase's invite flow never has a password: they enter via a
one-time invite-token session. Once that session expires, email+password login is
impossible, and nothing ever prompted them to set one. The enforcement machinery
(`PasswordChangeGuard()` in `internal/api/middleware/middleware.go`, the frontend
tenant-layout redirect, the change-password screen that clears the flag) was fully
built on both sides but never triggered, because `password_change_required` defaulted
to `false` and nothing ever flipped it to `true`.

## The change

`internal/api/usecases/invitation_usecase.go`, end of `ActivatePendingInvitations`
(previously ~line 246-251):

```go
if activated == 0 {
    return nil
}

return uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive)
```

became:

```go
if activated == 0 {
    return nil
}

// Invited users come from Supabase's invite flow and never set a password;
// once their one-time invite session expires, email+password login is
// impossible without one. Force the change-password screen on next load
// so they set one while we know they're still authenticated.
//
// Both writes are best-effort side effects of an otherwise-successful
// activation: run them independently (a failure in one must not skip the
// other) and join their errors. Like SetStatus below, the caller
// (JWTAuth) only logs this error and does not abort the request — the
// user is mid-login and already provisioned; failing the request over a
// secondary flag write would be worse than a user who occasionally
// doesn't get prompted to set a password until their next visit.
statusErr := uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive)
pwErr := uc.userRepo.SetPasswordChangeRequired(ctx, userID, true)
return errors.Join(statusErr, pwErr)
```

### Where and why

Placed directly after the `if activated == 0 { return nil }` guard — the exact spot
the task pointed at, and the only place in the function that knows "at least one
invitation really activated." Nothing upstream of that guard (expired invitations,
failed membership creation) reaches this code, so a call that activates nothing
never touches the flag.

### Error-handling choice, and why

I chose to call both repo writes unconditionally (not short-circuit after the first
error) and combine their results with `errors.Join` (Go 1.20+, available under
Go 1.24), then return that to the caller — the same contract `SetStatus` already had
before my change (the function's return value is the only channel back to the
caller).

I verified the caller's behavior directly rather than assuming it. In
`internal/api/middleware/middleware.go:96-113`:

```go
if activator != nil && user.Status == domain.UserStatusInvited && email != "" {
    if err := activator.ActivatePendingInvitations(ctx, email, user.ID); err != nil {
        Log.Warn("failed to activate pending invitations",
            zap.String("user_id", user.ID),
            zap.Error(err),
        )
    } else if refreshed, err := authUC.ProvisionUser(ctx, sub, email); err != nil {
        ...
    } else {
        user = refreshed
    }
}
```

On error, `JWTAuth` calls `Log.Warn(...)` and falls straight through — there is no
`c.Abort*` and no `return` in that branch. Execution continues into the rest of
`JWTAuth` (context is still populated, `c.Next()` still runs). So today, a `SetStatus`
failure already does not abort the request; it only leaves the user with a stale
`invited` status in context for that one request. My change preserves exactly that
contract for the combined error: a failure in either write is logged by the existing
call site and the request proceeds.

That is deliberate, not incidental: this call fires inside `JWTAuth`, on a request
that has already authenticated successfully and already provisioned/activated the
user's membership. Aborting login over a failure to write a secondary flag
(`password_change_required`) would turn a soft edge case (user isn't prompted to set
a password until a later visit, where `ActivatePendingInvitations` — no, see caveat
below — or another path could retry) into a hard outage for a real, legitimately
activated user. Given the flag's job is to prompt a screen, not to gate access by
itself (that's what `PasswordChangeGuard` does downstream, unaffected by this code
path), logging and proceeding is the safer failure mode. I did not invent this
policy — I confirmed it's the one already in force for `SetStatus` and extended it
consistently to the new write, using `errors.Join` so a failure in one write can't
silently swallow a failure in the other (previously fixable with two sequential
`if err != nil { return err }` blocks, but that would skip the second write whenever
the first fails, which is worse for this specific pair — both are meant to happen
together).

**Caveat worth flagging**: if either write fails, there's no automatic retry on a
later login, because invitations are already marked `accepted` and
`ListPendingByEmail` will return empty next time (`activated` stays `0`). This is a
pre-existing characteristic of the `SetStatus` half of this code (not introduced by
me) and now also applies to `SetPasswordChangeRequired`. It's an edge case, not a
regression, and out of scope for this task — worth a follow-up if these DB writes turn
out to be flaky in practice.

## Frontend

Not touched. The tenant layout's redirect (triggered by `/api/v1/me` reporting the
flag) and the change-password screen were already correct and complete; adding a
second enforcement point would have been a duplicated rule, which the task
explicitly said to avoid.

## Testing

A focused unit test was achievable with a reasonable number of small fakes, so I
wrote one: `internal/api/usecases/activate_pending_invitations_test.go`.

`InvitationUsecase` only needs 3 of its 6 dependencies to exercise this path
(`invRepo`, `userRepo`, `userRoleRepo` — `tenantRepo`, `roleRepo`, `supabaseClient`,
`redis` are unused by `ActivatePendingInvitations`), so I built minimal fakes for
those three interfaces (`fakeInvRepoForActivation`, `fakeUserRoleRepoForActivation`,
`fakeUserRepoForActivation`), following the fake style already used in
`invite_metadata_test.go` (plain structs implementing only the interface, recording
calls where the test needs to assert on them, returning zero values for methods this
code path never calls).

Three tests:

- `TestActivatePendingInvitations_ActivaYRequierePassword` — one pending,
  non-expired invitation activates successfully; asserts `SetStatus(active)` and
  `SetPasswordChangeRequired(true)` were both called exactly once, and the invitation
  was marked `accepted`.
- `TestActivatePendingInvitations_SinInvitacionesPendientesNoTocaFlag` — no pending
  invitations at all; asserts neither `SetStatus` nor `SetPasswordChangeRequired` was
  called.
- `TestActivatePendingInvitations_TodasExpiradasNoTocaFlag` — a pending invitation
  exists but is expired (covers the `activated == 0` path via a different route than
  "no invitations", since the expired-invitation branch marks it `expired` and
  `continue`s rather than skipping the loop entirely); asserts the same as above.

## Verification

Run in the foreground (Go isn't installed on the host; per repo convention, via
`golang:1.24-alpine` in Docker with a persistent module cache at
`/tmp/go-mod-cache`):

1. `go build ./... && go vet ./...` — clean, no output, exit 0.
2. `go test ./internal/api/usecases/... -v` — all 12 tests in the package pass,
   including the 3 new ones:
   ```
   --- PASS: TestActivatePendingInvitations_ActivaYRequierePassword (0.00s)
   --- PASS: TestActivatePendingInvitations_SinInvitacionesPendientesNoTocaFlag (0.00s)
   --- PASS: TestActivatePendingInvitations_TodasExpiradasNoTocaFlag (0.00s)
   ... (9 pre-existing tests in the package, all PASS)
   ok      github.com/tu-org/embolsadora-api/internal/api/usecases       0.015s
   ```
3. `go test ./internal/...` — every package with tests reports `ok`; no failures,
   no regressions anywhere in the module.

`gofmt -l` was run and the new test file was reformatted (a struct-field alignment
issue); the two other files it flagged (`password_usecase.go`,
`tenants/get_all_tenants_test.go`) are pre-existing and untouched by this change, so
left as-is.

## Self-review

- **Fires only on real activation?** Yes — placed after `activated == 0 { return
  nil }`, confirmed with the "all expired" test that this guard is reached even when
  the pending list was non-empty but nothing survived to activate.
- **Failure mode sane?** Yes, and verified against the actual caller rather than
  assumed — see the quoted `JWTAuth` code above. Log-and-continue, matching the
  existing `SetStatus` contract.
- **Frontend untouched?** Confirmed — no files outside `internal/api/usecases/` and
  this report were modified.

## Concerns

- The no-retry edge case described above (a DB failure on either write means the
  flag/status can only be fixed by a manual DB update or a fresh invitation, since
  the invitation is already `accepted`) is pre-existing behavior for `SetStatus` and
  now shared by `SetPasswordChangeRequired`. Not a regression, but worth knowing if
  these writes are ever observed to fail in production logs.
