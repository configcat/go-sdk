# Steps to deploy
## Preparation
1. Make sure the code is properly formatted.
   ```bash
   gofmt -s
   ```
2. Run tests
   ```bash
   go test
   ```
3. Increase the version in `version.go`
4. Commit & Push
## Publish
- Via git tag
    1. Create a new version tag.
       ```bash
       git tag v1.6
       ```

    2. Push the tag.
       ```bash
       git push origin --tags
       ```
- Via Github release 

  Create a new [Github release](https://github.com/configcat/go-sdk/releases) with a new version tag and release notes.

## Versioning
Pattern: `vMAJOR[.MINOR[.PATCH]]`.
Check https://gopkg.in/configcat/go-sdk.v4 for version resolution.

## Update import examples in local README.md

## Update import examples in Dashboard

## Update samples
Update and test sample apps with the new SDK version.