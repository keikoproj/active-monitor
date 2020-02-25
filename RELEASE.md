# Release Process

The Active-Monitor Project is released on an as-needed basis. The process is as follows:

1. An issue is created which proposes a new release with a changelog since the last release
1. All [OWNERS](.github/CODEOWNERS) are suggested to look at and sign off (ex: commenting with "LGTM") on this release
1. An [OWNER](.github/CODEOWNERS) updates [CHANGELOG](./CHANGELOG) with release details and updates badge at top of [README](./README.md)
1. A PR is created with these changes. Upon approval, it is merged to `master` branch.
1. Now, at `HEAD` on `master` branch, an [OWNER](.github/CODEOWNERS) runs `git tag -a $VERSION` and pushes the tag with `git push --tags`
1. The release complete!
1. Consumers can now pull docker image from [DockerHub](https://hub.docker.com/r/keikoproj/active-monitor/tags)

Note: This process does not apply to alpha/dev/latest (pre-)releases which may be cut at any time for development
and testing.

Note: This process adapted from that used in Kubebuilder project - https://raw.githubusercontent.com/kubernetes-sigs/kubebuilder/v2.2.0/RELEASE.md