# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
Bleeding-edge development, not yet released

## [0.10.0] - 2022-09-27
## Updated
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

[Unreleased]: https://github.com/keikoproj/active-monitor/compare/v0.9.0...HEAD
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
