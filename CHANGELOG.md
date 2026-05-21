<a name="Registry Go Mod Changelog"></a>

## Registry Module (in Go)
[Github repository](https://github.com/edgexfoundry/go-mod-registry)

## [v4.0.2] - 2026-05-31

### ✨ Features 
- Update to use `ServiceProtocol` as health check type ([#ed6e5be](https://github.com/edgexfoundry/go-mod-registry/commits/ed6e5be))

### ♻ Code Refactoring 
- Update count type to int64 ([#208](https://github.com/edgexfoundry/go-mod-registry/issues/208)) ([#938428f](https://github.com/edgexfoundry/go-mod-registry/commits/938428f))

### 👷 Build 
- Update goVersion to 1.25 and golangci-lint to 2.5.0 ([#209](https://github.com/edgexfoundry/go-mod-registry/issues/209)) ([#5af0325](https://github.com/edgexfoundry/go-mod-registry/commits/5af0325))


## [v4.0.0] - 2025-03-12
### ✨ Features

- Remove consul dependency ([8c20e08…](https://github.com/edgexfoundry/go-mod-registry/commit/8c20e088c5777c0639aa46125c70e2781a672c84))
```text

BREAKING CHANGE: Remove consul dependency

```
- Add Core Keeper registry client ([d2ed7e7…](https://github.com/edgexfoundry/go-mod-registry/commit/d2ed7e792f9cae09deddb7cc4f155098de030ee2))
```text

BREAKING CHANGE: Introduced Core Keeper as a new service for configuration and registry management

```
### ♻ Code Refactoring

- Update go module to v4 ([f858874…](https://github.com/edgexfoundry/go-mod-registry/commit/f85887483c2d8c593972c9098c95016830bfb7e3))
```text

BREAKING CHANGE: Update go module to v4

```

### 📖 Documentation

- Removed outdated installation instructions for using the module ([9d0c9bf…](https://github.com/edgexfoundry/go-mod-registry/commit/9d0c9bf73e160c4b1efa4c4e1efe5bb125249e55))

### 👷 Build

- Upgrade to go-1.23, Linter1.61.0 ([e5e47b3…](https://github.com/edgexfoundry/go-mod-registry/commit/e5e47b323af70f60e0679cfd866cafc9b01d70dd))


## [v3.1.0] - 2023-11-15


### 👷 Build

- Upgrade to go 1.21 and linter 1.54.2 ([57427c9…](https://github.com/edgexfoundry/go-mod-registry/commit/57427c9c3f686bf05ac839874c74e582eea689df))

## [v3.0.0] - 2023-05-31

### Code Refactoring ♻

- Update module to v3 ([#08a0459](https://github.com/edgexfoundry/go-mod-registry/commit/08a0459fb241432d7d1645e6d7d3539a588455c6))
  ```text
  BREAKING CHANGE: Import paths will need to change to v3
  ```

### Build 👷

- Update to Go 1.20 and linter v1.51.2 ([#be5d5bf](https://github.com/edgexfoundry/go-mod-registry/commits/be5d5bf))

## [v2.3.0] - 2022-11-09

### Features ✨

- None

### Build 👷

- Upgrade to Go 1.18 ([#7102501](https://github.com/edgexfoundry/go-mod-registry/commits/7102501))

## [v2.2.0] - 2022-05-11

### Features ✨

- None

### Build 🔄

- **security:** Enable gosec and default linter set ([#4863bf5](https://github.com/edgexfoundry/go-mod-registry/commits/4863bf5))
## [v2.1.0] - 2021-11-17

### Test

- Update Client interface mock for unit test (GetAllServiceEndpoints) ([#9d684a7](https://github.com/edgexfoundry/go-mod-registry/commits/9d684a7))

### Features ✨

- Add Renew Access Token capability ([#d344f9d](https://github.com/edgexfoundry/go-mod-registry/commits/d344f9d))
- Add the new GetAllServiceEndpoints API to the Client interface ([#c5087a8](https://github.com/edgexfoundry/go-mod-registry/commits/c5087a8))
- Add new GetAllServiceEndpoints method to retrieve all registered service endpoints from consul ([#d798a43](https://github.com/edgexfoundry/go-mod-registry/commits/d798a43))

## [v2.0.0] - 2021-06-30
### Features ✨
- **security:** Add ability to provide ACL AccessToken ([#5a93214](https://github.com/edgexfoundry/go-mod-registry/commits/5a93214))

<a name="v0.1.27"></a>
## [v0.1.27] - 2020-12-11
### Features ✨
- **registry:** Additional Check Registration ([#c5c016b](https://github.com/edgexfoundry/go-mod-registry/commits/c5c016b))

<a name="v0.1.20"></a>
## [v0.1.20] - 2020-03-25
### Bug
- **race:** Resolved race conditions. ([#837193e](https://github.com/edgexfoundry/go-mod-registry/commits/837193e))

<a name="v0.1.17"></a>
## [v0.1.17] - 2020-01-29
### Code Refactoring ♻
- **registry:** Refactor out all Configuration related APIs ([#5d84a90](https://github.com/edgexfoundry/go-mod-registry/commits/5d84a90))

