# Steps to deploy
## Preparation
1. Go to the current major versions folder.
   ```bash
   cd v<major>
   ```
2. Make sure the code is properly formatted.
   ```bash
   gofmt -s
   ```
3. Run tests
   ```bash
   go test
   ```
4. Increase the version in `version.go`
5. When you want to increase the major version, create a new `v<major>` folder and copy all the sources from the previous major versions folder and start the work there.
6. Commit & Push
## Publish
- Via git tag
    1. Create a new version tag.
       ```bash
       git tag v1.6.4
       ```

    2. Push the tag.
       ```bash
       git push origin --tags
       ```
- Via Github release 

  Create a new [Github release](https://github.com/configcat/go-sdk/releases) with a new version tag and release notes.

## Update import examples in local README.md

## Update import examples in Dashboard

## Update samples
Update and test sample apps with the new SDK version.