# Changelog

All notable changes to this project will be documented in this file. See [commit-and-tag-version](https://github.com/absolute-version/commit-and-tag-version) for commit guidelines.

## [0.1.0-alpha.3](https://github.com/gsantos-hc/vault-secrets-cleanup/compare/v0.1.0-alpha.2...v0.1.0-alpha.3) (2026-04-29)

## [0.1.0-alpha.2](https://github.com/gsantos-hc/vault-secrets-cleanup/compare/v0.1.0-alpha.1...v0.1.0-alpha.2) (2026-04-29)


### Features

* **discover:** add parallel workers ([#11](https://github.com/gsantos-hc/vault-secrets-cleanup/issues/11)) ([7ba64ef](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/7ba64efd2674d56dacaca9cff436666649faa820))
* **execute:** delete secrets in parallel ([a52c722](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/a52c722c1e20b12b2df302cf487b2a7f46881b4e))
* **execute:** save deletion progress ([#9](https://github.com/gsantos-hc/vault-secrets-cleanup/issues/9)) ([700fe0f](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/700fe0fd97641ae06fe823b58587cd19f76789b5))
* **seed:** add parallel execution ([#10](https://github.com/gsantos-hc/vault-secrets-cleanup/issues/10)) ([f8f0452](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/f8f0452877727c2d29bf0f4e8347e756bad7b5d7))

## [0.1.0-alpha.1](https://github.com/gsantos-hc/vault-secrets-cleanup/compare/v0.1.0-alpha.0...v0.1.0-alpha.1) (2026-04-28)


### Features

* **analyze:** add audit analysis progress reporting ([b1f270e](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/b1f270e77487687dabd18f431d800bd249462bc3))
* **discover:** add discovery progress tracking and CLI reporting ([de92009](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/de920098965f2a2d753c4258fb33bb2d68db16dc))
* **execute:** add deletion execution progress callbacks and reporting ([9027c42](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/9027c420fe2d89d6dc1f119d505c361cfa999091))
* **import:** add import aggregation progress reporting ([de3745b](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/de3745bfebadec0b391dde5b219e89584b40e39d))
* **plan:** add correlation and planning progress reporting ([71fe783](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/71fe783c70b3b27f9917d7b8b2fb79230fd96da7))
* **progress:** add global progress mode and shared reporter infrastructure ([9a579b4](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/9a579b4ef57ed43627db348e3bf0c7154a322819))
* **seed:** add seeding progress callbacks and script reporter ([3c47078](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/3c470782b8e058387f0b3d4a7a396c4d15679651))

## 0.1.0-alpha.0 (2026-04-27)


### Features

* add audit processing ([3b0db01](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/3b0db0103816b8bb9a7ab9a8124af2fd66f79907))
* add deletion engine ([25a1da8](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/25a1da801259e6a98b4cc93cdefc8da4cdacb490))
* add deletion plans ([1d07841](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/1d07841e3734fb25a91409e6039613b50d99395f))
* add secrets discovery ([c224728](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/c22472811138049f2e03a7dabd19676f4c33dd34))
* add vault seeding script ([de211b2](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/de211b289b1f0ff6f3c7dea95dd463d9b1f797cc))
* implement phase 1 foundation ([69b6635](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/69b66356a3794b8b7c57641c62d6d4f6b8ac0618))
* **importer:** add support for data from jq filter ([4756cc9](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/4756cc90629a95316f98bc923d0fe7c7d1626f8e))
* sub-day time parsing ([a0b935b](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/a0b935b3cf6c7703f5372b9967a79e355c465310))


### Bug Fixes

* **seed:** add mount limit per ns ([0a29d36](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/0a29d36fe97246faf9726761957e4dd29f6b75a2))
* strip api paths from secret names ([c192fcf](https://github.com/gsantos-hc/vault-secrets-cleanup/commit/c192fcf3f627e9b0568d08991f022ac9b6b10064))
