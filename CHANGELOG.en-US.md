# Changelog
All notable changes to choerodon-cluster-agent will be documented in this file.

## [0.15.0] - 2019-03-22
### Added
- imagePullSecret supported

## [0.9.0] - 2018-08-17
### Added
- Implement GitOps
- Add label of k8s resource before apply and install helm release

## [0.8.0] - 2018-07-20
### Added
- Job event listener.

### Changed
- Change the default tail lines of pod log.

### Fixed
- Remove useless timestamp in pod logs.

## [0.7.0] - 2018-06-29
### Fixed
- Unchecked cases when WebSocket component parameters are inadequate. And the resulting failure in connection soon after connection established.
