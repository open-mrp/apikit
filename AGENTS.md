# AGENTS.md

This file provides guidance when working with code in this repository.

## What this repo is

apikit is the generic HTTP API framework extracted from `open-mrp/api`. It is a library: no services, no database schema, no deploys. Applications import its subpackages and register their own identity type, authorizer, error codes, ID prefixes, API versions and sensitive-field policies.

The public API contract it implements is forge.1, defined in `open-mrp/api` at `docs/patterns/public-api-design-conventions.md`. If code here and that doc disagree, the doc wins.

## Rules

- **Nothing OpenMRP-specific.** No domain constants, protos, business error codes, permission models or brand names (such as an `OpenMRP-Version` header). Anything an app needs to vary is config or a registration.
- **This repository is public.** Never commit customer names, IDs, emails, or values copied from a production record, in code, tests, fixtures, commit messages or PR text.
- **Breaking changes are `feat!:`.** release-please versions from Conventional Commits; while below v1, a breaking change bumps the minor version.
- Every package has tests, and moved code keeps the tests it had in `open-mrp/api`.

## Commands

```sh
go build ./...
go test ./...
go vet ./...
staticcheck ./...
```
