# Releasing

End-to-end checklist for cutting `vX.Y.Z`.

## 1. Prepare the release branch

```sh
git switch -c release/vX.Y.Z
```

- Bump any version constants in source (if added in future).
- Update `CHANGELOG.md` (Keep-a-Changelog format) once it exists.

## 2. Refresh the embedded UI

```sh
cd web
bun install --frozen-lockfile
bun run build
cd ..
git add web/dist
```

`web/dist/` is committed so `go install` users (who don't run `bun`) still get the latest UI. Rebuild whenever the SPA source changes.

## 3. Local checks

```sh
go test ./...
golangci-lint-v2 run            # optional but recommended
```

## 4. PR, review, merge

Open the release branch as a PR, get review, merge to `master`.

## 5. Tag

```sh
git switch master
git pull
git tag -s vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Signed tags are preferred but not enforced.

## 6. Run the archive workflow

GitHub → Actions → **release** → Run workflow → input the tag (`vX.Y.Z`).

This builds six archives (`linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64`, `windows_amd64`, `windows_arm64`) plus a `huetension_<version>_checksums.txt` file and publishes a **draft** GitHub release.

Cosign-signed checksums are produced when the repo has `COSIGN_OIDC_PROVIDER` set as a repository variable; otherwise the signing step is skipped.

## 7. Verify the archives

Download one archive, extract it, and run the binary:

```sh
tar -xzf huetension_X.Y.Z_linux_amd64.tar.gz
./huetension version       # should print X.Y.Z
./huetension web --address 127.0.0.1:8080
```

Confirm the SPA loads at <http://127.0.0.1:8080> and the sidebar shows the new version.

## 8. Publish the draft release

In the GitHub UI, flip the draft to published.

## 9. Run the container workflow

GitHub → Actions → **container** → Run workflow → tag (`vX.Y.Z`), `publish_latest = true`.

This builds and pushes `ghcr.io/leporel/huetension:vX.Y.Z` and (if requested) `:latest` for `linux/amd64` and `linux/arm64`.

The container workflow is independent of the archive workflow — either can be retried without re-running the other.

## 10. Smoke test `go install`

From a clean GOPATH:

```sh
go install github.com/leporel/huetension/cmd/huetension@vX.Y.Z
huetension version
```

The version output should match the tag (without the `v` prefix).

## Hotfix releases

For `vX.Y.(Z+1)`:

1. Branch from the tag, not from `master`: `git switch -c hotfix/vX.Y.Z+1 vX.Y.Z`.
2. Apply the fix, run steps 2–10.
3. Forward-port the fix to `master` with a separate PR.
