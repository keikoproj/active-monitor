# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
Bleeding-edge development, not yet released

## [0.12.0] - 2023-09-21
### Updated
* Bump github.com/emicklei/go-restful from 2.9.6+incompatible to 2.16.0+incompatible by @dependabot in https://github.com/keikoproj/active-monitor/pull/135
* Bump golang.org/x/text from 0.3.2 to 0.3.8 by @dependabot in https://github.com/keikoproj/active-monitor/pull/137
* Bump golang.org/x/crypto from 0.0.0-20190820162420-60c769a6c586 to 0.1.0 by @dependabot in https://github.com/keikoproj/active-monitor/pull/139
* Bump github.com/gogo/protobuf from 1.2.2-0.20190723190241-65acae22fc9d to 1.3.2 by @dependabot in https://github.com/keikoproj/active-monitor/pull/148
* Bump gopkg.in/yaml.v3 from 3.0.0-20200615113413-eeeca48fe776 to 3.0.0 by @dependabot in https://github.com/keikoproj/active-monitor/pull/147
* Bump golang.org/x/net from 0.0.0-20191004110552-13f9640d40b9 to 0.7.0 by @dependabot in https://github.com/keikoproj/active-monitor/pull/138
* Bump github.com/prometheus/client_golang from 1.0.0 to 1.11.1 by @dependabot in https://github.com/keikoproj/active-monitor/pull/136
* Bump k8s.io/client-go from 0.17.2 to 0.17.16 by @dependabot in https://github.com/keikoproj/active-monitor/pull/149
* Update to Golang 1.19 by @tekenstam in https://github.com/keikoproj/active-monitor/pull/150
* chore: upgrade kubernetes dependencies to support upto k8s 1.25 by @rkilingr in https://github.com/keikoproj/active-monitor/pull/146
* Fix dependabot config by @tekenstam in https://github.com/keikoproj/active-monitor/pull/153
* Remove Travis CI config and update badges in README by @tekenstam in https://github.com/keikoproj/active-monitor/pull/165
* Bump actions/checkout from 2 to 4 by @dependabot in https://github.com/keikoproj/active-monitor/pull/154
* Bump docker/setup-buildx-action from 1 to 3 by @dependabot in https://github.com/keikoproj/active-monitor/pull/157
* Bump docker/build-push-action from 2 to 5 by @dependabot in https://github.com/keikoproj/active-monitor/pull/158
* Bump docker/setup-qemu-action from 1 to 3 by @dependabot in https://github.com/keikoproj/active-monitor/pull/159
* Update to docker/metadata-action@v5 by @tekenstam in https://github.com/keikoproj/active-monitor/pull/166
* Improve the Dependabot configuration by @tekenstam in https://github.com/keikoproj/active-monitor/pull/167
* Bump github.com/keikoproj/inverse-exp-backoff from 0.0.0-20201007213207-e4a3ac0f74ab to 0.0.3 by @dependabot in https://github.com/keikoproj/active-monitor/pull/161
* Bump golang.org/x/net from 0.10.0 to 0.15.0 by @dependabot in https://github.com/keikoproj/active-monitor/pull/160
* Bump github.com/sirupsen/logrus from 1.9.2 to 1.9.3 by @dependabot in https://github.com/keikoproj/active-monitor/pull/163
* Bump github.com/argoproj/argo-workflows/v3 from 3.4.8 to 3.4.11 by @dependabot in https://github.com/keikoproj/active-monitor/pull/164
* Bump docker/login-action from 1 to 3 by @dependabot in https://github.com/keikoproj/active-monitor/pull/168
* Bump actions/setup-go from 2 to 4 by @dependabot in https://github.com/keikoproj/active-monitor/pull/169

### New Contributors
* @dependabot made their first contribution in https://github.com/keikoproj/active-monitor/pull/135
* @tekenstam made their first contribution in https://github.com/keikoproj/active-monitor/pull/150

**Full Changelog**: https://github.com/keikoproj/active-monitor/compare/v0.11.2...v0.12.0

## [0.11.2] - 2023-04-05
## Updated
- feat: enable multi-architecture container image builds - #144

## [0.11.1] - 2023-03-06
## Fixed
- bugfix: Fixes timer replacement without clearing the old one present - #141

## [0.11.0] - 2022-12-07
## Updated
- Update workflow-controller and argoexec version to v3.4.4 - #132
## Fixed
- Rbac and optimistic lock issues - #129

## [0.10.0] - 2022-09-27
## Fixed
- Fix remedyWorkflow deleting RBAC and Service Account created from external system - #122

## [0.9.0] - 2022-09-01
## Updated
- Update workflow-controller and argo-exec version to v3.3.9 - #117

## [0.8.0] - 2022-08-18
## Updated
- Update active-monitor CRD from v1beta1 to v1 - #105
- Update go version to 1.18 - #111
### Fixed
- Fix CHANGELOG file - #110
- Fix after upgrade argo BDD fails with errors - #109

## [0.7.0] - 2022-03-11
## Updated
- Update Argo workflow controller version to v3.2.6 - #107

## [0.6.0] - 2021-06-17
### Added
- Move to Github Actions for CI enhancement - #81
## Updated
-  Update Default TTL Strategy to secondsAfterCompletion - #99
-  Update Argo controller version - #80
### Fixed
-   Active Monitor crashing with concurrent map updates - #98

## [0.5.2] - 2021-05-18
### Fixed
- Active Monitor crashing with concurrent map updates - #88

## [0.5.1] - 2021-03-05
### Fixed
- active-monitor running workflows more frequently than the configuration - #82

## [0.5.0] - 2020-12-23
### Added
- Exponentially reduce Kubernetes API calls- #64
- Limit number of times the Self-Healing/Remedy should be run - #65
- Enable default PodGC strategy as OnPodCompletion in workflow - #67
- Add Events to Acive-Monitor Custom Resources - #70

## [0.4.0] - 2020-08-04
### Added
- Consolidate Metric Endpoints for Active-Monitor - #7
- Update Status fields to include Total Healthcheck count - #49
- Update healthcheck spec and controller to support automatic "remediation" - #23
- Active-Monitor to process multiple custom resources in parallel - #54
### Updated
- Update Controller-gen to v0.2.4 and kube-builder to v2.3.0. - #47
### Fixed
- Workflow creation ignores metadata information in the healthcheck spec - #55

## [0.3.0] - 2020-02-24
### Added
- Support cron-like expression for designating healthcheck frequency - #22

## [0.2.0] - 2020-02-14
### Added
- Healthcheck status columns in `kubectl get ...` printed output - #31
- Addl healthcheck metrics for start+end times of latest run - #32
- Support for cluster or namespace scoping - #12
- Documentation improvements - #3

### Fixed
- improved reliability of controller by wrapping in a recover block - #30

## [0.1.0] - 2019-08-09
### Added
- Initial commit of project

[Unreleased]: https://github.com/keikoproj/active-monitor/compare/v0.11.2...HEAD
[0.11.2]: https://github.com/keikoproj/active-monitor/compare/v0.11.1...v0.11.2
[0.11.1]: https://github.com/keikoproj/active-monitor/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/keikoproj/active-monitor/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/keikoproj/active-monitor/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/keikoproj/active-monitor/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/keikoproj/active-monitor/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/keikoproj/active-monitor/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/keikoproj/active-monitor/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/keikoproj/active-monitor/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/keikoproj/active-monitor/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/keikoproj/active-monitor/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/keikoproj/active-monitor/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/keikoproj/active-monitor/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/keikoproj/active-monitor/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/keikoproj/active-monitor/releases/tag/v0.1.0
