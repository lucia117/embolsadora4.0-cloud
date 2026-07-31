# PR #53 code review fixes — write-order bug and dead loggers

## Finding 1 — write order made a failed activation permanent

### The bug

`internal/api/usecases/invitation_usecase.go`, end of `ActivatePendingInvitations`:

```go
statusErr := uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive)
pwErr := uc.userRepo.SetPasswordChangeRequired(ctx, userID, true)
return errors.Join(statusErr, pwErr)
```

The only caller, `JWTAuth` (`internal/api/middleware/middleware.go:96`), only invokes this
function while the user is still `invited`:

```go
if activator != nil && user.Status == domain.UserStatusInvited && email != "" {
```

If `SetStatus` succeeded and `SetPasswordChangeRequired` failed, the user was left `active`
forever with `password_change_required == false`. Nothing else in the codebase ever sets
that flag for invitation-flow users, so that invitee would never be prompted to set a
password, and once their one-time Supabase invite session expired, email+password login
would be permanently impossible for them.

### The fix

```go
if err := uc.userRepo.SetPasswordChangeRequired(ctx, userID, true); err != nil {
    return err
}
return uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive)
```

Setting the flag first and gating the status flip on its success means: if the flag write
fails, the function returns before `SetStatus` runs, so the user stays `invited`. The
`JWTAuth` gate (`user.Status == domain.UserStatusInvited`) is still true on the next
authenticated request, so the whole activation — including membership creation — retries
automatically. This is genuinely self-healing: there is no window where the gate is false
but the flag is unset, because the status can only become `active` after the flag write
already succeeded.

One subtlety worth naming: on retry, `ListPendingByEmail` may return nothing if all
invitations were already marked `accepted` in a previous partial run. In that case
`activated` stays `0` and the function returns `nil` without touching `SetStatus` or the
flag — so the user remains `invited` and login stays blocked rather than silently
succeeding half-activated. That's an existing property of the loop (untouched by this fix)
and it's the conservative failure mode, not a new gap.

### Comment rewrite

The old comment claimed the worst case was "a user who occasionally doesn't get prompted
to set a password until their next visit" — false, because there is no next visit once the
gate closes. Replaced with an explanation of why the order matters (see code below).

```go
// Invited users come from Supabase's invite flow and never set a password;
// once their one-time invite session expires, email+password login is
// impossible without one. Force the change-password screen on next load
// so they set one while we know they're still authenticated.
//
// Order matters here and it is not arbitrary: JWTAuth only calls this
// function while user.Status == 'invited' (see middleware.go), so once
// SetStatus below succeeds, this code never runs again for this user. If
// we flipped the status first and the flag write then failed, the user
// would be stuck 'active' with password_change_required stuck false —
// permanently, since nothing else ever sets that flag for invitation-flow
// users. Setting the flag first and gating the status flip on its success
// means a failure here leaves the user 'invited', so the next
// authenticated request retries the whole activation instead of leaving a
// user who can never log in with email+password.
if err := uc.userRepo.SetPasswordChangeRequired(ctx, userID, true); err != nil {
    return err
}
return uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive)
```

### `errors` import

`errors.Join` is gone, but `errors.Is` is still used twice elsewhere in the file
(`CreateInvitation` and the membership-create branch of `ActivatePendingInvitations`), so
the `"errors"` import stays — no unused-import cleanup needed.

### New test

Added `TestActivatePendingInvitations_FallaFlagNoActivaUsuario` to
`internal/api/usecases/activate_pending_invitations_test.go`, proving the self-healing
property directly:

- Gave `fakeUserRepoForActivation` a new field `setPasswordChangeRequiredErr`, returned by
  its `SetPasswordChangeRequired` method (previously it always returned `nil`).
- Test sets that field to `assert.AnError`, runs `ActivatePendingInvitations` with one
  valid pending invitation, and asserts:
  - the returned error wraps `assert.AnError` (`require.ErrorIs`)
  - `userRepo.setStatusCalls` is **empty** — `SetStatus` was never called
  - `SetPasswordChangeRequired` was called exactly once, with `true`

This is the concrete evidence that a flag-write failure never lets the status flip.

## Finding 2 — package-level loggers were always no-op

### The bug

`internal/api/middleware/middleware.go:25` and `internal/api/usecases/invitation_usecase.go:21`
both declared:

```go
var Log *zap.Logger = zap.NewNop()
```

The middleware.go comment claimed "Set via SetLogger during application startup" but no
such function existed anywhere in the repo (confirmed via `grep -rn "SetLogger"` returning
only that comment itself, before this fix). `internal/routes/url_mappings.go` built a real
`*zap.Logger` via `zap.NewDevelopment()` and wired it into `passwordUC`, `dlService`,
`rService`, etc., but never into the two package globals. Result: every `Log.*` call added
in this PR — including the one in
`internal/api/handler/invitations/create_invitation/create_invitation.go` whose own comment
says it exists to close "el punto ciego que hacia invisible el bug de la URL" — was silent.

### The fix

Added an identical `SetLogger` function to both packages:

```go
// SetLogger replaces the package-level logger. A nil argument is ignored so
// callers can't accidentally silence logging by passing an uninitialized
// logger.
func SetLogger(l *zap.Logger) {
    if l == nil {
        return
    }
    Log = l
}
```

And wired both calls into `internal/routes/url_mappings.go`, immediately after the logger
is constructed and before any use case, handler, or middleware is built or wired:

```go
logger, err := zap.NewDevelopment()
if err != nil {
    log.Fatalf("failed to initialize logger: %v", err)
}
apimw.SetLogger(logger)
usecases.SetLogger(logger)
```

Both `apimw` and `usecases` were already imported in that file, so no import changes were
needed there.

Updated the middleware.go comment to describe reality instead of aspiration:

```go
// Log is the package-level Zap logger. Defaults to a no-op logger; call
// SetLogger during application startup (before any middleware runs) to wire
// in a real one.
var Log *zap.Logger = zap.NewNop()
```

(Mirrored analogously in `usecases`: "Defaults to a no-op logger; call SetLogger during
application startup to wire in a real one.")

### Self-review: are both calls reached before anything logs?

Yes. `RegisterURLMappings` is the only place that constructs use cases, handlers, and
middleware for this API (`cmd/api/main.go` calls it once). The `SetLogger` calls sit right
after `logger, err := zap.NewDevelopment()` and before:
- `authUC`, `invUC`, `passwordUC` are constructed (usecases package logging)
- `apimw.JWTAuth`, `apimw.TenantFromHeader`, etc. are wired into route groups (middleware
  package logging)
- any handler that could call `usecases.Log` (e.g. `create_invitation.go`) is registered

There is no code path in `RegisterURLMappings` that logs through either global before these
two lines execute.

## Verification

Go isn't installed on the host; ran via Docker per repo convention
(`golang:1.24-alpine`, module cache at `/tmp/go-mod-cache`), in the foreground:

```
go build ./... && go vet ./... && go test ./internal/...
```

Result: build and vet produced no output (clean), and every package with tests reported
`ok`, including:

```
ok  	github.com/tu-org/embolsadora-api/internal/api/middleware	0.009s
ok  	github.com/tu-org/embolsadora-api/internal/api/usecases	0.013s
```

Ran the four `ActivatePendingInvitations` tests individually with `-v` for direct evidence:

```
=== RUN   TestActivatePendingInvitations_ActivaYRequierePassword
--- PASS: TestActivatePendingInvitations_ActivaYRequierePassword (0.00s)
=== RUN   TestActivatePendingInvitations_FallaFlagNoActivaUsuario
--- PASS: TestActivatePendingInvitations_FallaFlagNoActivaUsuario (0.00s)
=== RUN   TestActivatePendingInvitations_SinInvitacionesPendientesNoTocaFlag
--- PASS: TestActivatePendingInvitations_SinInvitacionesPendientesNoTocaFlag (0.00s)
=== RUN   TestActivatePendingInvitations_TodasExpiradasNoTocaFlag
--- PASS: TestActivatePendingInvitations_TodasExpiradasNoTocaFlag (0.00s)
PASS
ok  	github.com/tu-org/embolsadora-api/internal/api/usecases	0.011s
```

`gofmt -l ./internal/` was also run; none of the four files touched by this change
(`internal/api/middleware/middleware.go`, `internal/api/usecases/invitation_usecase.go`,
`internal/api/usecases/activate_pending_invitations_test.go`,
`internal/routes/url_mappings.go`) appear in its output. The files it does flag are
pre-existing formatting drift, untouched by this PR.

## The `zap.NewDevelopment()` observation — confirmed, not fixed

Read `internal/routes/url_mappings.go:85` and `internal/config/config.go`. Confirmed the
reading is accurate:

- `zap.NewDevelopment()` is called unconditionally, with no branch on `cfg.Env`
  (`Environment`, values include `local`/`beta`/`production` per `internal/config/config.go`)
  or `cfg.Observability.LogLevel` (also present in config, read from `LOG_LEVEL` env var,
  default `"info"`).
- `zap.NewDevelopment()` produces human-readable console-format output (caller info, stack
  traces on Warn+, colorized level names), not JSON.
- On Cloud Run, log ingestion parses structured JSON lines into queryable fields
  (severity, jsonPayload, etc.); a development-formatter logger writing plain text lines in
  production means Finding 2's fix makes logs *emit* but they still won't get structured
  ingestion in Cloud Run — they'll show up as unparsed text blobs.

This is pre-existing (not introduced by PR #53) and out of scope for these two findings, so
it was left unchanged. Flagging it here so it can be tracked separately: the config already
carries the two signals (`Env`, `Observability.LogLevel`) needed to pick `zap.NewProduction()`
(or a custom JSON encoder config with the configured level) outside of `local`.

## Files changed

- `internal/api/usecases/invitation_usecase.go` — write-order fix, comment rewrite,
  `SetLogger` added.
- `internal/api/middleware/middleware.go` — `SetLogger` added, comment corrected.
- `internal/routes/url_mappings.go` — both `SetLogger` calls wired in at startup.
- `internal/api/usecases/activate_pending_invitations_test.go` — new
  `setPasswordChangeRequiredErr` fake field + `TestActivatePendingInvitations_FallaFlagNoActivaUsuario`.

## Concerns

- None outstanding for the two findings themselves — both fixes are narrow, tested, and
  verified end-to-end (build/vet/test all green).
- The `zap.NewDevelopment()` / Cloud Run JSON formatting gap (documented above) should be
  raised as a separate follow-up; it means Finding 2's fix makes the new logs visible but
  not yet ideally structured for querying in production.
