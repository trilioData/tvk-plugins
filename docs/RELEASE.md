   # Release Guidelines

This guide intends to outline process to release a new versions of tvk-plugins.

## Background:

Release process for both plugins is handled by goreleaser CI utility. Release packages include tarball and sha256 text file 
for both plugins. What packages should be released depends on code changes in both plugins' code and it's calculated
based on file changes between current tag and previous tag([`refer`](hack/check-git-diff-between-tags.sh)). 

## Steps:

1. Set Release Tag:

    Follow [semantic versioning](https://semver.org/spec/v2.0.0.html). Release tags versions should start with `v`. Ex. v1.0.0
    ```
    TAG=v1.0.0
    ```

2. Create Release branch:

    Follow branch naming convention like `release/vx.x.x`
    ```
    git checkout -b release/$TAG origin/main
    ```
   
3.  Tag the Release:

     ```
     git tag -a -m "msg" "${TAG:?TAG required}"  
     ```
4. Push the release branch & tag:

     ```
     git push origin release/$TAG
     git push origin "${TAG:?TAG required}"    
     ```
    or 
    ```
    git push origin release/$TAG --tags
    ```
   
5. Wait until the github actions workflow `Plugin Packages CI` succeeds.

6. Verify on Releases tab on GitHub. Make sure plugin's tarball and sha256 release assets show up on `Releases` tab and
   `Releases` is marked as `pre-release`(not ready for production).
   
7. Perform QA on release packages using testing methods mentioned in [`CONTRIBUTION.md`](docs/CONTRIBUTION.md). [optional]

8. Once release build is verified, update plugin manifests using methods mentioned in [`CONTRIBUTION.md`](docs/CONTRIBUTION.md)
   and create PR for the same.
   
9. Wait for github actions workflow `Plugin Manifests CI` to succeeds for newly created PR containing plugin manifest changes, merge PR
   once workflows succeeds.

10. From Github `Releases` tab, update Release's CHANGELOG and uncheck `pre-release` and update release.

11. Now, Release is ready for production and will be marked as latest release for github.
