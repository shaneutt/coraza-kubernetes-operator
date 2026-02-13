# Releases

This documentation will help you build and publish a new release of the Coraza
Kubernetes Operator (CKO).

> **Note**: All releases target tags, and our tags follow [semver].

> **Note**: Most of the release process is automated via [GitHub Workflow]. See
> the [release.yml] workflow for details.

[semver]:https://github.com/semver/semver
[GitHub Workflow]:https://docs.github.com/en/actions/concepts/workflows-and-actions/workflows
[release.yml]:https://github.com/networking-incubator/coraza-kubernetes-operator/blob/main/.github/workflows/release.yml

## Process

### Step 1 - Communication

Confirm with all other maintainers the plans to cut a release.

This should generally coincide with the completion of one of our [milestones]
for any major or minor releases.

> **Note**: Patch releases may be cut at any time out of `main` or another
> branch depending on the criticality of the patches included.

[milestones]:https://github.com/networking-incubator/coraza-kubernetes-operator/milestones

### Step 2 - Tag

Create a tag off the top of the `main` branch, e.g.:

```console
git tag v0.1.1
```

Push the tag to the repository, e.g.:

```console
git push upstream v0.1.1
```

This will trigger workflows to test and create the release:

- `build-test`
- `release`

You can follow along on the [actions page].

Release will cut a `draft` release from the tag, however stop here and wait to
make sure `build-test` is successful for this tag before proceeding.

> **Note**: tags that start with `v0` or have suffixes including `rc`, `alpha`,
> or `beta` (e.g. `v0.1.1`, `v1.0.0-rc1`, `v0.1.0-alpha1`) will be
> automatically marked as _pre-releases_.

[actions page]:https://github.com/networking-incubator/coraza-kubernetes-operator/actions

### Step 3 - Validation & Publishing

> **Warning**: We enforce [immutable releases] so be _absolutely certain_ the
> tests are passing and the release is ready before you publish it.

Once you've confirmed the `build-test` workflow has succeeded for this tag,
review the `draft` release for your tag on the [releases page]. Verify the
following are correct:

* The release **name** should just be the tag name
* The release **description** should include the auto-generated release notes
* The **crds.yaml**, **operator.yaml** & **samples.yaml** artifacts are attached
  * Check each manifest, and verify its correctness
* Make sure the **previous release** is set correctly
  * e.g. for a `v1.0.0` release, _don't_ target `rc` or other pre-releases

Once you've verified the release integrity, publish the release.

[releases page]:https://github.com/networking-incubator/coraza-kubernetes-operator/releases

### Step 4 - Announcement

The release page will ask you if you want to create a discussion to announce
the release. Either say yes to that and publish an `announcement` type
discussion for the release that links to the release page, or go to the
discussions page and write up an announcement from there.

Make sure the latest release announcement is pinned, and older release
announcements get unpinned.

[immutable releases]:https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases
